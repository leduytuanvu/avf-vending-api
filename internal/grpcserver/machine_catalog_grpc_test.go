package grpcserver

import (
	"context"
	"fmt"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"strings"
	"sync"
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineA, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
				MachineID: id,
				SiteID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Items:     nil,
				Bootstrap: &setupapp.MachineBootstrap{},
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
		pid := id.NewUUIDV7()
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(mid, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
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

type mutexSnapCatalog struct {
	mu   sync.Mutex
	snap salecatalog.Snapshot
}

func (m *mutexSnapCatalog) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts salecatalog.Options) (salecatalog.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.snap
	out.MachineID = machineID
	out.GeneratedAt = time.Now().UTC()
	if out.Bootstrap == nil {
		b := setupapp.MachineBootstrap{}
		out.Bootstrap = &b
	}
	return out, nil
}

func catalogBundleTestSnapshot(displayHash string) salecatalog.Snapshot {
	b := setupapp.MachineBootstrap{}
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	return salecatalog.Snapshot{
		SiteID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		ConfigVersion: 1,
		Currency:      "THB",
		Bootstrap:     &b,
		Items: []salecatalog.Item{{
			ProductID:         pid,
			SKU:               "SKU1",
			Name:              "Water",
			SlotIndex:         1,
			IsAvailable:       true,
			AvailableQuantity: 3,
			MaxQuantity:       10,
			PriceMinor:        100,
			BasePriceMinor:    100,
			Image: &salecatalog.ImageMeta{
				MediaID:      mid,
				MediaVersion: 1,
				ContentHash:  "sha256:" + displayHash,
				ThumbURL:     "https://cdn.example/t.webp",
				DisplayURL:   "https://cdn.example/d.webp",
				ContentType:  "image/webp",
				SizeBytes:    100,
				UpdatedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
				Variants: []salecatalog.ImageVariantMeta{
					{
						Kind: salecatalog.MediaVariantKindDisplay, MediaAssetID: mid,
						URL: "https://cdn.example/d.webp", StorageKey: "k/d",
						ChecksumSHA256: "sha256:" + displayHash, Etag: `W/"d"`,
						ContentType: "image/webp", SizeBytes: 100, MediaVersion: 1,
						UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
		}},
	}
}

func TestMachineCatalog_SyncCatalogBundle_FirstSyncReturnsProductsAndMedia(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	snap := catalogBundleTestSnapshot("aaa")
	stub := stubSaleCatalog{snap: snap}

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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetChangedProducts()) != 1 {
		t.Fatalf("changed_products=%d", len(resp.GetChangedProducts()))
	}
	if len(resp.GetChangedMediaAssets()) != 1 {
		t.Fatalf("changed_media_assets=%d", len(resp.GetChangedMediaAssets()))
	}
	a := resp.GetChangedMediaAssets()[0]
	if a.GetDownloadUrl() == "" || a.GetSha256() == "" || a.GetVersion() == 0 {
		t.Fatalf("media asset: %#v", a)
	}
	if resp.GetCatalogVersion() == "" || resp.GetMediaManifestVersion() == "" {
		t.Fatal("expected non-empty version cursors")
	}
}

func TestMachineCatalog_SyncCatalogBundle_SecondSyncSameVersionsEmpty(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	snap := catalogBundleTestSnapshot("aaa")
	opts := salecatalog.Options{IncludeUnavailable: false, IncludeImages: true}
	catVer := salecatalog.CatalogSyncCatalogVersion(*snap.Bootstrap, snap, opts)
	mediaVer := salecatalog.MediaFingerprint(snap)
	stub := stubSaleCatalog{snap: snap}

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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{
		CurrentCatalogVersion:       catVer,
		CurrentMediaManifestVersion: mediaVer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetChangedProducts()) != 0 || len(resp.GetChangedMediaAssets()) != 0 {
		t.Fatalf("expected empty deltas: products=%d media=%d", len(resp.GetChangedProducts()), len(resp.GetChangedMediaAssets()))
	}
	if resp.GetMeta().GetStatus() != machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_NOT_MODIFIED {
		t.Fatalf("expected not_modified meta: %v", resp.GetMeta().GetStatus())
	}
}

func TestMachineCatalog_SyncCatalogBundle_MediaOnlyDelta(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	snapA := catalogBundleTestSnapshot("aaa")
	snapB := catalogBundleTestSnapshot("bbb")
	opts := salecatalog.Options{IncludeUnavailable: false, IncludeImages: true}
	if salecatalog.CatalogSyncCatalogVersion(*snapA.Bootstrap, snapA, opts) != salecatalog.CatalogSyncCatalogVersion(*snapB.Bootstrap, snapB, opts) {
		t.Fatal("fixture: catalog sync versions must match for media-only delta case")
	}
	if salecatalog.MediaFingerprint(snapA) == salecatalog.MediaFingerprint(snapB) {
		t.Fatal("fixture: media versions must differ")
	}

	mc := &mutexSnapCatalog{snap: snapA}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: mc, Pool: nil},
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	r1, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	mc.mu.Lock()
	mc.snap = snapB
	mc.mu.Unlock()

	r2, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{
		CurrentCatalogVersion:       r1.GetCatalogVersion(),
		CurrentMediaManifestVersion: r1.GetMediaManifestVersion(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.GetChangedProducts()) != 0 {
		t.Fatalf("expected no product delta, got %d", len(r2.GetChangedProducts()))
	}
	if len(r2.GetChangedMediaAssets()) != 1 {
		t.Fatalf("expected media delta: %v", r2.GetChangedMediaAssets())
	}
}

func TestMachineCatalog_SyncCatalogBundle_MissingMediaNotSellable(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	b := setupapp.MachineBootstrap{}
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	stub := stubSaleCatalog{
		snap: salecatalog.Snapshot{
			SiteID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Bootstrap: &b,
			Items: []salecatalog.Item{{
				ProductID:         pid,
				SKU:               "SKU1",
				Name:              "X",
				SlotIndex:         1,
				IsAvailable:       false,
				UnavailableReason: "missing_primary_media",
				Image: &salecatalog.ImageMeta{
					Deleted:   true,
					UpdatedAt: time.Now().UTC(),
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	resp, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	it := resp.GetChangedProducts()[0]
	if it.GetIsAvailable() || !strings.Contains(it.GetUnavailableReason(), "missing_primary_media") {
		t.Fatalf("expected missing media readiness: %#v", it)
	}
	if len(resp.GetChangedMediaAssets()) != 0 {
		t.Fatal("deleted primary should not emit media assets")
	}
}

func TestMachineCatalog_SyncCatalogBundle_BasisRemovals(t *testing.T) {
	t.Parallel()
	cfg := testMachineGRPCConfig()
	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	snap := catalogBundleTestSnapshot("aaa")
	stale := id.NewUUIDV7().String()
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{
			deps: MachineGRPCServicesDeps{SaleCatalog: stubSaleCatalog{snap: snap}, Pool: nil},
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
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
	cli := machinev1.NewMachineCatalogServiceClient(conn)
	mid := snap.Items[0].Image.MediaID.String()
	resp, err := cli.SyncCatalogBundle(md, &machinev1.SyncCatalogBundleRequest{
		BasisProductIds:     []string{stale},
		BasisMediaAssetKeys: []string{mid + ":display", stale + ":thumb"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetRemovedProductIds()) != 1 || resp.GetRemovedProductIds()[0] != stale {
		t.Fatalf("removed products: %v", resp.GetRemovedProductIds())
	}
	wantKey := stale + ":thumb"
	if len(resp.GetRemovedMediaAssetIds()) != 1 || resp.GetRemovedMediaAssetIds()[0] != wantKey {
		t.Fatalf("removed media: %v", resp.GetRemovedMediaAssetIds())
	}
}

func TestProductMediaRefProto_externalURLMetadata(t *testing.T) {
	t.Parallel()
	mid := id.NewUUIDV7()
	im := &salecatalog.ImageMeta{
		MediaID:          mid,
		ThumbURL:         "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
		DisplayURL:       "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
		ContentType:      "image/png",
		MediaVersion:     1,
		SourceType:       "external_url",
		CacheKey:         "external-image:abc:v1",
		OfflineRequired:  true,
		DownloadStrategy: "download_when_online_use_local_when_offline",
		UpdatedAt:        time.Now().UTC(),
	}
	pm := productMediaRefProto(im)
	if pm == nil {
		t.Fatal("nil ref")
	}
	if pm.GetMediaId() != mid.String() {
		t.Fatalf("media id: %s", pm.GetMediaId())
	}
	if pm.GetSourceType() != "external_url" {
		t.Fatalf("source type: %s", pm.GetSourceType())
	}
	if pm.GetCacheKey() != im.CacheKey {
		t.Fatalf("cache key: %s", pm.GetCacheKey())
	}
	if !pm.GetOfflineRequired() {
		t.Fatal("offline required")
	}
	if pm.GetDownloadStrategy() != im.DownloadStrategy {
		t.Fatalf("download strategy: %s", pm.GetDownloadStrategy())
	}
}
