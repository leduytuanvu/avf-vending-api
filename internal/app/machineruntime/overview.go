package machineruntime

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var machineCodePattern = regexp.MustCompile(`^AVF[0-9]{6,}$`)

// OverviewFilter filters fleet operational overview rows.
type OverviewFilter struct {
	SiteID       *uuid.UUID
	MachineID    *uuid.UUID
	OnlineStatus string
	MachineCode  string
	Lifecycle    string
	Limit        int32
	Offset       int32
}

// AdminOperationalOverview is the enriched admin machine ops snapshot.
type AdminOperationalOverview struct {
	MachineID           uuid.UUID       `json:"machineId"`
	MachineCode         string          `json:"machineCode"`
	MachineName         string          `json:"machineName"`
	LifecycleStatus     string          `json:"lifecycleStatus"`
	OnlineStatus        string          `json:"onlineStatus"`
	SaleEnabled         bool            `json:"saleEnabled"`
	MachineType         string          `json:"machineType,omitempty"`
	LastSeenAt          *time.Time      `json:"lastSeenAt,omitempty"`
	CredentialVersion   int64           `json:"credentialVersion"`
	SiteID              uuid.UUID       `json:"siteId"`
	SiteName            string          `json:"siteName"`
	AndroidBoard        *AndroidBoard   `json:"androidBoard,omitempty"`
	SIM                 *SIMInfo        `json:"sim,omitempty"`
	RuntimeAppSession   *AppSessionView `json:"runtimeAppSession,omitempty"`
	CredentialSession   *CredentialView `json:"credentialSession,omitempty"`
	OperatorSession     *OperatorView   `json:"operatorSession,omitempty"`
	FinalSellReady      bool            `json:"finalSellReady"`
	Readiness           json.RawMessage `json:"readiness,omitempty"`
	Connectivity        json.RawMessage `json:"connectivity,omitempty"`
}

// AndroidBoard is a safe view of the active device attachment.
type AndroidBoard struct {
	AttachmentID  uuid.UUID `json:"attachmentId"`
	AndroidID     string    `json:"androidId,omitempty"`
	BoardSerial   string    `json:"boardSerial,omitempty"`
	DeviceSerial  string    `json:"deviceSerial,omitempty"`
	Manufacturer  string    `json:"manufacturer,omitempty"`
	Model         string    `json:"model,omitempty"`
	AppBuildSHA   string    `json:"appBuildSha,omitempty"`
	PackageName   string    `json:"packageName,omitempty"`
	VersionName   string    `json:"versionName,omitempty"`
	AttachedAt    time.Time `json:"attachedAt,omitempty"`
	Reason        string    `json:"reason,omitempty"`
}

// SIMInfo captures SIM identity from the active attachment.
type SIMInfo struct {
	ICCID     string `json:"iccid,omitempty"`
	Operator  string `json:"operator,omitempty"`
	CountryISO string `json:"countryIso,omitempty"`
	Serial    string `json:"serial,omitempty"`
}

// AppSessionView is the true app runtime lifecycle session (not credential JWT session).
type AppSessionView struct {
	SessionID         uuid.UUID       `json:"sessionId"`
	Status            string          `json:"status"`
	StartReason       string          `json:"startReason,omitempty"`
	StartedAt         time.Time       `json:"startedAt"`
	LastHeartbeatAt   *time.Time      `json:"lastHeartbeatAt,omitempty"`
	LastMQTTSSeenAt   *time.Time      `json:"lastMqttSeenAt,omitempty"`
	LastMQTTState     string          `json:"lastMqttState,omitempty"`
	StorefrontState   string          `json:"storefrontState,omitempty"`
	SellReady         bool            `json:"sellReady"`
	Blockers          json.RawMessage `json:"blockers,omitempty"`
}

// CredentialView is machine_sessions (JWT credential session).
type CredentialView struct {
	SessionID         uuid.UUID  `json:"sessionId"`
	Status            string     `json:"status"`
	IssuedAt          time.Time  `json:"issuedAt"`
	CredentialVersion int64      `json:"credentialVersion"`
}

// OperatorView is the active human operator session.
type OperatorView struct {
	SessionID uuid.UUID `json:"sessionId"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
}

// NormalizeMachineCode uppercases and strips whitespace for AVF code search.
func NormalizeMachineCode(code string) string {
	return strings.ToUpper(regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(code), ""))
}

// ValidMachineCode reports whether code matches ^AVF[0-9]{6,}$.
func ValidMachineCode(code string) bool {
	return machineCodePattern.MatchString(NormalizeMachineCode(code))
}

// BuildMachineAdminOperationalOverview returns enriched overview for one machine.
func (s *Service) BuildMachineAdminOperationalOverview(ctx context.Context, machineID uuid.UUID) (AdminOperationalOverview, error) {
	if s == nil || s.q == nil {
		return AdminOperationalOverview{}, errors.New("machineruntime: nil service")
	}
	rows, err := s.q.AdminListMachineOperationalOverview(ctx, overviewListParams(OverviewFilter{
		MachineID: &machineID,
		Limit:     1,
	}))
	if err != nil {
		return AdminOperationalOverview{}, err
	}
	if len(rows) == 0 {
		return AdminOperationalOverview{}, pgx.ErrNoRows
	}
	return mapOverviewRow(ctx, s.q, rows[0])
}

// ListMachineAdminOperationalOverview lists fleet overview rows with filters.
func (s *Service) ListMachineAdminOperationalOverview(ctx context.Context, f OverviewFilter) ([]AdminOperationalOverview, int64, error) {
	if s == nil || s.q == nil {
		return nil, 0, errors.New("machineruntime: nil service")
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}
	rows, err := s.q.AdminListMachineOperationalOverview(ctx, overviewListParams(f))
	if err != nil {
		return nil, 0, err
	}
	cnt, err := s.q.AdminCountMachineOperationalOverview(ctx, overviewCountParams(f))
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminOperationalOverview, 0, len(rows))
	for _, row := range rows {
		ov, err := mapOverviewRow(ctx, s.q, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, ov)
	}
	return out, cnt, nil
}

func overviewListParams(f OverviewFilter) db.AdminListMachineOperationalOverviewParams {
	p := db.AdminListMachineOperationalOverviewParams{
		Column5:  false,
		Column7:  false,
		Column9:  false,
		LimitVal: f.Limit,
		OffsetVal: f.Offset,
	}
	if f.SiteID != nil {
		p.Column1 = true
		p.Column2 = *f.SiteID
	}
	if f.MachineID != nil {
		p.Column3 = true
		p.Column4 = *f.MachineID
	}
	if st := strings.TrimSpace(f.OnlineStatus); st != "" {
		p.Column5 = true
		p.Column6 = st
	}
	if code := strings.TrimSpace(f.MachineCode); code != "" {
		p.Column7 = true
		norm := NormalizeMachineCode(code)
		if !strings.Contains(norm, "%") {
			norm = norm + "%"
		}
		p.Column8 = norm
	}
	if lc := strings.TrimSpace(f.Lifecycle); lc != "" {
		p.Column9 = true
		p.Column10 = lc
	}
	return p
}

func overviewCountParams(f OverviewFilter) db.AdminCountMachineOperationalOverviewParams {
	p := db.AdminCountMachineOperationalOverviewParams{
		Column5: false,
		Column7: false,
		Column9: false,
	}
	if f.SiteID != nil {
		p.Column1 = true
		p.Column2 = *f.SiteID
	}
	if f.MachineID != nil {
		p.Column3 = true
		p.Column4 = *f.MachineID
	}
	if st := strings.TrimSpace(f.OnlineStatus); st != "" {
		p.Column5 = true
		p.Column6 = st
	}
	if code := strings.TrimSpace(f.MachineCode); code != "" {
		p.Column7 = true
		norm := NormalizeMachineCode(code)
		if !strings.Contains(norm, "%") {
			norm = norm + "%"
		}
		p.Column8 = norm
	}
	if lc := strings.TrimSpace(f.Lifecycle); lc != "" {
		p.Column9 = true
		p.Column10 = lc
	}
	return p
}

func mapOverviewRow(ctx context.Context, q *db.Queries, row db.AdminListMachineOperationalOverviewRow) (AdminOperationalOverview, error) {
	online, _ := computeOnlineFromRow(row)
	out := AdminOperationalOverview{
		MachineID:         row.MachineID,
		MachineCode:       row.MachineCode,
		MachineName:       row.MachineName,
		LifecycleStatus:   row.LifecycleStatus,
		OnlineStatus:      online,
		SaleEnabled:       row.SaleEnabled,
		MachineType:       pgTextVal(row.MachineType),
		CredentialVersion: row.CredentialVersion,
		SiteID:            row.SiteID,
		SiteName:          row.SiteName,
	}
	if row.LastSeenAt.Valid {
		t := row.LastSeenAt.Time.UTC()
		out.LastSeenAt = &t
	}
	if row.DeviceAttachmentID.Valid {
		id := uuid.UUID(row.DeviceAttachmentID.Bytes)
		out.AndroidBoard = &AndroidBoard{
			AttachmentID: id,
			AndroidID:    pgTextVal(row.AndroidID),
			BoardSerial:  pgTextVal(row.BoardSerial),
		}
	}
	if row.SimIccid.Valid || row.SimOperator.Valid {
		out.SIM = &SIMInfo{
			ICCID:    pgTextVal(row.SimIccid),
			Operator: pgTextVal(row.SimOperator),
		}
	}
	if row.RuntimeAppSessionID.Valid {
		id := uuid.UUID(row.RuntimeAppSessionID.Bytes)
		sellReady := row.SellReady.Valid && row.SellReady.Bool
		sv := &AppSessionView{
			SessionID:       id,
			Status:          pgTextVal(row.RuntimeSessionStatus),
			StartReason:     pgTextVal(row.RuntimeStartReason),
			StorefrontState: pgTextVal(row.StorefrontState),
			SellReady:       sellReady,
		}
		if row.RuntimeStartedAt.Valid {
			sv.StartedAt = row.RuntimeStartedAt.Time.UTC()
		}
		if row.RuntimeLastHeartbeatAt.Valid {
			t := row.RuntimeLastHeartbeatAt.Time.UTC()
			sv.LastHeartbeatAt = &t
		}
		if row.LastMqttSeenAt.Valid {
			t := row.LastMqttSeenAt.Time.UTC()
			sv.LastMQTTSSeenAt = &t
		}
		sv.LastMQTTState = pgTextVal(row.LastMqttState)
		if len(row.Blockers) > 0 {
			sv.Blockers = json.RawMessage(row.Blockers)
		}
		out.RuntimeAppSession = sv
		out.FinalSellReady = sellReady && strings.EqualFold(row.LifecycleStatus, "active") && row.SaleEnabled
	}
	if row.CredentialSessionID.Valid {
		id := uuid.UUID(row.CredentialSessionID.Bytes)
		cv := &CredentialView{
			SessionID:         id,
			Status:            pgTextVal(row.CredentialSessionStatus),
			CredentialVersion: row.CredentialVersion,
		}
		if row.CredentialIssuedAt.Valid {
			cv.IssuedAt = row.CredentialIssuedAt.Time.UTC()
		}
		out.CredentialSession = cv
	}
	if row.OperatorSessionID.Valid {
		id := uuid.UUID(row.OperatorSessionID.Bytes)
		ov := &OperatorView{
			SessionID: id,
			Status:    pgTextVal(row.OperatorSessionStatus),
		}
		if row.OperatorStartedAt.Valid {
			ov.StartedAt = row.OperatorStartedAt.Time.UTC()
		}
		out.OperatorSession = ov
	}
	conn, _ := json.Marshal(map[string]any{
		"onlineStatus": online,
		"lastSeenAt":   out.LastSeenAt,
	})
	out.Connectivity = conn
	if out.RuntimeAppSession != nil && len(out.RuntimeAppSession.Blockers) > 0 {
		out.Readiness = json.RawMessage(out.RuntimeAppSession.Blockers)
	}
	return out, nil
}

func computeOnlineFromRow(row db.AdminListMachineOperationalOverviewRow) (string, error) {
	st := strings.TrimSpace(row.OnlineStatus)
	if st == "" {
		st = "unknown"
	}
	return st, nil
}

func pgTextVal(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return strings.TrimSpace(t.String)
}

// DeviceIdentityFromFingerprint maps activation fingerprint JSON to attach identity.
func DeviceIdentityFromFingerprint(fp json.RawMessage, clientIP, userAgent string, meta json.RawMessage) DeviceIdentity {
	var m map[string]any
	_ = json.Unmarshal(fp, &m)
	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}
	var sdk *int32
	if v, ok := m["sdkInt"]; ok {
		switch n := v.(type) {
		case float64:
			i := int32(n)
			sdk = &i
		}
	}
	var vc *int64
	if v, ok := m["versionCode"]; ok {
		switch n := v.(type) {
		case float64:
			i := int64(n)
			vc = &i
		}
	}
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	return DeviceIdentity{
		AndroidID:      get("androidId", "android_id"),
		AndroidSerial:  get("androidSerial", "android_serial"),
		BoardSerial:    get("boardSerial", "board_serial"),
		DeviceSerial:   get("serialNumber", "device_serial", "deviceSerial"),
		SimSerial:      get("simSerial", "sim_serial"),
		SimICCID:       get("simIccid", "sim_iccid"),
		SimOperator:    get("simOperator", "sim_operator"),
		SimCountryISO:  get("simCountryIso", "sim_country_iso"),
		Manufacturer:   get("manufacturer"),
		Brand:          get("brand"),
		Model:          get("model"),
		DeviceModel:    get("deviceModel", "device_model"),
		Hardware:       get("hardware"),
		Product:        get("product"),
		AndroidRelease: get("androidRelease", "android_release"),
		SDKInt:         sdk,
		PackageName:    get("packageName", "package_name"),
		VersionName:    get("versionName", "version_name"),
		VersionCode:    vc,
		AppBuildSHA:    get("appBuildSha", "app_build_sha"),
		BootID:         get("bootId", "boot_id"),
		NetworkType:    get("networkType", "network_type"),
		NetworkState:   get("networkState", "network_state"),
		IPAddress:      clientIP,
		UserAgent:      userAgent,
		Metadata:       meta,
	}
}
