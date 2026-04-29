package grpcserver

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubSaleCatalog struct {
	snap salecatalog.Snapshot
	err  error
}

func (s stubSaleCatalog) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts salecatalog.Options) (salecatalog.Snapshot, error) {
	if s.err != nil {
		return salecatalog.Snapshot{}, s.err
	}
	out := s.snap
	out.MachineID = machineID
	out.GeneratedAt = time.Now().UTC()
	if out.Bootstrap == nil {
		b := setupapp.MachineBootstrap{}
		out.Bootstrap = &b
	}
	return out, nil
}

func TestMachineCatalog_GetCatalogSnapshot_CrossMachineDenied(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	machineB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  3,
			CatalogVersion: "abc",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID:   uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				SKU:         "SKU1",
				Name:        "Soda",
				IsAvailable: true,
				Image: &salecatalog.ImageMeta{
					ThumbURL:   "https://cdn.example/t.webp",
					DisplayURL: "https://cdn.example/d.webp",
				},
			}},
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{
				SaleCatalog: stub,
				Pool:        nil,
			},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineA, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	_, err = cli.GetCatalogSnapshot(md, &machinev1.GetCatalogSnapshotRequest{MachineId: machineB.String()})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got %v: %v", status.Code(err), err)
	}
}

func TestMachineCatalog_GetCatalogSnapshot_ReturnsURLsNotBinary(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "ver",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID:   pid,
				SKU:         "SKU1",
				Name:        "Water",
				IsAvailable: true,
				Image: &salecatalog.ImageMeta{
					ThumbURL:    "https://cdn.example/t.webp",
					DisplayURL:  "https://cdn.example/d.webp",
					ContentHash: "sha256:deadbeef",
					Etag:        `W/"deadbeef"`,
				},
			}},
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{
				SaleCatalog: stub,
				Pool:        nil,
			},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.GetCatalogSnapshot(md, &machinev1.GetCatalogSnapshotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	snap := resp.GetSnapshot()
	if snap == nil {
		t.Fatal("nil snapshot")
	}
	if len(snap.GetItems()) != 1 {
		t.Fatalf("items=%d", len(snap.GetItems()))
	}
	pm := snap.GetItems()[0].GetPrimaryMedia()
	if pm == nil || pm.GetThumbUrl() == "" {
		t.Fatal("expected media urls")
	}
	for _, u := range []string{pm.GetThumbUrl(), pm.GetDisplayUrl()} {
		if !strings.HasPrefix(u, "https://") {
			t.Fatalf("expected https image reference, got %q (no binary payloads in catalog)", u)
		}
	}
}

type fnSaleCatalog struct {
	fn func(context.Context, uuid.UUID, salecatalog.Options) (salecatalog.Snapshot, error)
}

func (f fnSaleCatalog) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts salecatalog.Options) (salecatalog.Snapshot, error) {
	return f.fn(ctx, machineID, opts)
}

func TestMachineCatalog_GetCatalogSnapshot_IncludeUnavailablePassesThrough(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	var got salecatalog.Options
	snapFn := fnSaleCatalog{
		fn: func(ctx context.Context, id uuid.UUID, opts salecatalog.Options) (salecatalog.Snapshot, error) {
			got = opts
			return salecatalog.Snapshot{
				MachineID:      id,
				OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Items:          nil,
				Bootstrap:      &setupapp.MachineBootstrap{},
			}, nil
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: snapFn, Pool: nil},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	_, err = cli.GetCatalogSnapshot(md, &machinev1.GetCatalogSnapshotRequest{IncludeUnavailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.IncludeUnavailable {
		t.Fatal("expected IncludeUnavailable true")
	}
}

func TestMachineCatalog_GetMediaManifest_ResourceExhausted(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	cfg.Capacity.MaxMediaManifestEntries = 64
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	items := make([]salecatalog.Item, 65)
	for i := range items {
		pid := uuid.New()
		items[i] = salecatalog.Item{
			ProductID: pid,
			SKU:       fmt.Sprintf("S%d", i),
			Name:      "X",
			Image: &salecatalog.ImageMeta{
				MediaID:    pid,
				ThumbURL:   "https://cdn.example/t.webp",
				DisplayURL: "https://cdn.example/d.webp",
				UpdatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
	}

	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "fp",
			Currency:       "THB",
			Items:          items,
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{
				SaleCatalog: stub,
				Pool:        nil,
				Config:      cfg,
			},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	_, err = cli.GetMediaManifest(md, &machinev1.GetMediaManifestRequest{})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v: %v", status.Code(err), err)
	}
}

func TestMachineCatalog_GetMediaManifest_IncludesChecksumAndMediaID(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mediaID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "ver",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID:   pid,
				SKU:         "SKU1",
				Name:        "Water",
				IsAvailable: true,
				Image: &salecatalog.ImageMeta{
					MediaID:       mediaID,
					ThumbURL:      "https://cdn.example/t.webp",
					DisplayURL:    "https://cdn.example/d.webp",
					ContentHash:   "sha256:deadbeef",
					Etag:          `W/"deadbeef"`,
					SizeBytes:     999,
					ObjectVersion: 7,
					MediaVersion:  3,
					UpdatedAt:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					Variants: []salecatalog.ImageVariantMeta{
						{Kind: salecatalog.MediaVariantKindDisplay, MediaAssetID: mediaID, URL: "https://cdn.example/d.webp", ChecksumSHA256: "sha256:deadbeef", Etag: `W/"deadbeef"`, SizeBytes: 999, MediaVersion: 3, UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
					},
				},
			}},
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{
				SaleCatalog: stub,
				Pool:        nil,
			},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.GetMediaManifest(md, &machinev1.GetMediaManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetEntries()) != 1 {
		t.Fatalf("entries=%d", len(resp.GetEntries()))
	}
	ent := resp.GetEntries()[0]
	if ent.GetProductId() != pid.String() || ent.GetMediaId() != mediaID.String() {
		t.Fatalf("entry ids: product=%q media=%q", ent.GetProductId(), ent.GetMediaId())
	}
	pm := ent.GetPrimaryMedia()
	if pm == nil || pm.GetChecksumSha256() == "" || pm.GetThumbUrl() == "" {
		t.Fatal("expected primary media urls and checksum")
	}
	if pm.GetMediaId() != mediaID.String() || pm.GetSizeBytes() != 999 || pm.GetObjectVersion() != 7 {
		t.Fatalf("unexpected ref: %#v", pm)
	}
	if len(pm.GetMediaVariants()) != 1 || pm.GetMediaVariants()[0].GetChecksumSha256() == "" {
		t.Fatalf("expected media_variants with checksum: %#v", pm.GetMediaVariants())
	}
}

func TestMachineCatalog_GetCatalogDelta_BasisMatches(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			CatalogVersion: "match-ver",
			Items:          []salecatalog.Item{},
		},
	}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: stub, Pool: nil},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	mid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(mid, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.GetCatalogDelta(md, &machinev1.GetCatalogDeltaRequest{
		BasisCatalogVersion: "match-ver",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetBasisMatches() {
		t.Fatalf("expected basis_matches: %#v", resp)
	}
}

func TestMachineMedia_GetMediaManifestAndDelta(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mediaID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "ver",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID: pid,
				SKU:       "SKU1",
				Image: &salecatalog.ImageMeta{
					MediaID:      mediaID,
					ThumbURL:     "https://cdn.example/t.webp",
					DisplayURL:   "https://cdn.example/d.webp",
					ContentHash:  "sha256:deadbeef",
					MediaVersion: 1,
					UpdatedAt:    time.Now().UTC(),
				},
			}},
		},
	}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineMediaServiceServer(s, &machineMediaServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: stub, Pool: nil},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineMediaServiceClient(conn)
	resp, err := cli.GetMediaManifest(md, &machinev1.MachineMediaServiceGetMediaManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetEntries()) != 1 || resp.GetEntries()[0].GetMediaId() != mediaID.String() {
		t.Fatalf("unexpected media manifest: %#v", resp.GetEntries())
	}
	if resp.GetEntries()[0].GetPrimaryMedia().GetDisplayUrl() == "" {
		t.Fatal("expected URL metadata, not binary payload")
	}
	delta, err := cli.GetMediaDelta(md, &machinev1.GetMediaDeltaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.GetBasisMatches() {
		t.Fatal("expected basis mismatch with empty prior fingerprint")
	}
	if len(delta.GetChangedEntries()) != 1 {
		t.Fatalf("expected changed media delta rows: %#v", delta.GetChangedEntries())
	}

	fp := salecatalog.MediaFingerprint(stub.snap)
	delta2, err := cli.GetMediaDelta(md, &machinev1.GetMediaDeltaRequest{BasisMediaFingerprint: fp})
	if err != nil {
		t.Fatal(err)
	}
	if !delta2.GetBasisMatches() || len(delta2.GetChangedEntries()) != 0 {
		t.Fatalf("expected basis match: %#v", delta2)
	}
}

func TestMachineMedia_GetMediaManifest_DeletedTombstone(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "ver",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID: pid,
				SKU:       "SKU1",
				Image: &salecatalog.ImageMeta{
					Deleted:   true,
					UpdatedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
				},
			}},
		},
	}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineMediaServiceServer(s, &machineMediaServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: stub, Pool: nil},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineMediaServiceClient(conn)
	resp, err := cli.GetMediaManifest(md, &machinev1.MachineMediaServiceGetMediaManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetEntries()) != 1 {
		t.Fatalf("entries=%d", len(resp.GetEntries()))
	}
	pm := resp.GetEntries()[0].GetPrimaryMedia()
	if pm == nil || !pm.GetDeleted() {
		t.Fatalf("expected deleted tombstone, got %#v", pm)
	}
}

func TestMachineCatalog_GetMediaManifest_MediaVariantsChecksumAndExpires(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mediaID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	exp := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ConfigVersion:  1,
			CatalogVersion: "ver",
			Currency:       "THB",
			Items: []salecatalog.Item{{
				ProductID:   pid,
				SKU:         "SKU1",
				Name:        "Water",
				IsAvailable: true,
				Image: &salecatalog.ImageMeta{
					MediaID:      mediaID,
					ThumbURL:     "https://cdn.example/t.webp",
					DisplayURL:   "https://cdn.example/d.webp",
					ContentHash:  "sha256:deadbeef",
					Etag:         `W/"deadbeef"`,
					MediaVersion: 2,
					UpdatedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					URLExpiresAt: exp,
					Variants: []salecatalog.ImageVariantMeta{
						{Kind: salecatalog.MediaVariantKindThumb, MediaAssetID: mediaID, URL: "https://cdn.example/t.webp", ChecksumSHA256: "sha256:thumb", Etag: `W/"t"`, MediaVersion: 2, UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
						{Kind: salecatalog.MediaVariantKindDisplay, MediaAssetID: mediaID, URL: "https://cdn.example/d.webp", ContentType: "image/webp", ChecksumSHA256: "sha256:display", Etag: `W/"d"`, MediaVersion: 2, UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
					},
				},
			}},
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: stub, Pool: nil},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	conn, err := grpc.DialContext(context.Background(), srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.GetMediaManifest(md, &machinev1.GetMediaManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ent := resp.GetEntries()[0]
	pm := ent.GetPrimaryMedia()
	if len(pm.GetMediaVariants()) != 2 {
		t.Fatalf("media_variants: %v", pm.GetMediaVariants())
	}
	for _, mv := range pm.GetMediaVariants() {
		if mv.GetChecksumSha256() == "" || mv.GetUrl() == "" {
			t.Fatalf("variant missing url/checksum: %#v", mv)
		}
		if mv.GetExpiresAt() == nil || !mv.GetExpiresAt().AsTime().Equal(exp) {
			t.Fatalf("expected expires_at %v, got %v", exp, mv.GetExpiresAt())
		}
	}
}
