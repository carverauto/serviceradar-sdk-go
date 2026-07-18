package sdk

import "time"

const (
	// CapabilityAdvisoryFeedV1 marks plugins that emit normalized advisory
	// batches. Provider-specific download and parsing stay inside the plugin.
	CapabilityAdvisoryFeedV1 = "advisory-feed:v1"

	AdvisoryFeedContractVersion = "serviceradar.advisory_feed.contract.v1"

	CoordinateTypePURL          = "purl"
	CoordinateTypeCPE           = "cpe"
	CoordinateTypeVendorProduct = "vendor_product"
)

// AdvisoryFeedBatch is the normalized vulnerability intelligence payload that
// a plugin or add-on submits through the ServiceRadar plugin result contract.
type AdvisoryFeedBatch struct {
	SchemaVersion string           `json:"schema_version"`
	ProducerID    string           `json:"producer_id"`
	Source        AdvisorySource   `json:"source"`
	Snapshot      AdvisorySnapshot `json:"snapshot"`
	Advisories    []AdvisoryRecord `json:"advisories"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

type AdvisorySource struct {
	Provider               string         `json:"provider"`
	FeedKey                string         `json:"feed_key"`
	DisplayName            string         `json:"display_name,omitempty"`
	FeedType               string         `json:"feed_type,omitempty"`
	Enabled                bool           `json:"enabled"`
	URL                    string         `json:"url,omitempty"`
	SchemaURL              string         `json:"schema_url,omitempty"`
	RefreshIntervalSeconds int            `json:"refresh_interval_seconds,omitempty"`
	RetentionDays          int            `json:"retention_days,omitempty"`
	CredentialRef          string         `json:"credential_ref,omitempty"`
	Options                map[string]any `json:"options,omitempty"`
	LastMessage            string         `json:"last_message,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type AdvisorySnapshot struct {
	ObjectKey      string         `json:"object_key"`
	SHA256         string         `json:"sha256"`
	SourceURL      string         `json:"source_url,omitempty"`
	ContentType    string         `json:"content_type,omitempty"`
	Format         string         `json:"format,omitempty"`
	SizeBytes      int64          `json:"size_bytes,omitempty"`
	StorageBackend string         `json:"storage_backend,omitempty"`
	Accepted       bool           `json:"accepted"`
	Status         string         `json:"status,omitempty"`
	Error          string         `json:"error,omitempty"`
	Validation     map[string]any `json:"validation,omitempty"`
	FetchedAt      time.Time      `json:"fetched_at,omitempty"`
	AcceptedAt     time.Time      `json:"accepted_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type AdvisoryRecord struct {
	SourceObjectID      string               `json:"source_object_id"`
	AdvisoryID          string               `json:"advisory_id"`
	CVEID               string               `json:"cve_id,omitempty"`
	Title               string               `json:"title,omitempty"`
	Description         string               `json:"description,omitempty"`
	Severity            string               `json:"severity,omitempty"`
	CVSSScore           float64              `json:"cvss_score,omitempty"`
	CVSSVector          string               `json:"cvss_vector,omitempty"`
	PublishedAt         time.Time            `json:"published_at,omitempty"`
	ModifiedAt          time.Time            `json:"modified_at,omitempty"`
	KEV                 bool                 `json:"kev,omitempty"`
	ExploitAvailable    bool                 `json:"exploit_available,omitempty"`
	AffectedCoordinates []AffectedCoordinate `json:"affected_coordinates"`
	References          []string             `json:"references,omitempty"`
	Metadata            map[string]any       `json:"metadata,omitempty"`
}

type AffectedCoordinate struct {
	Type           string           `json:"type"`
	Value          string           `json:"value,omitempty"`
	Vendor         string           `json:"vendor,omitempty"`
	Product        string           `json:"product,omitempty"`
	MatchSemantics string           `json:"match_semantics,omitempty"`
	VersionRange   map[string]any   `json:"version_range,omitempty"`
	VersionRanges  []map[string]any `json:"version_ranges,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
}

func NewAdvisoryFeedBatch(producerID string, source AdvisorySource, snapshot AdvisorySnapshot) AdvisoryFeedBatch {
	return AdvisoryFeedBatch{
		SchemaVersion: AdvisoryFeedContractVersion,
		ProducerID:    producerID,
		Source:        source,
		Snapshot:      snapshot,
	}
}

func (b *AdvisoryFeedBatch) AddAdvisory(advisory AdvisoryRecord) {
	if b == nil {
		return
	}
	if b.SchemaVersion == "" {
		b.SchemaVersion = AdvisoryFeedContractVersion
	}
	b.Advisories = append(b.Advisories, advisory)
}

func (b AdvisoryFeedBatch) WithAdvisory(advisory AdvisoryRecord) AdvisoryFeedBatch {
	b.AddAdvisory(advisory)
	return b
}

func (r *Result) AddAdvisoryFeed(batch AdvisoryFeedBatch) {
	if batch.SchemaVersion == "" {
		batch.SchemaVersion = AdvisoryFeedContractVersion
	}

	r.AdvisoryFeed = append(r.AdvisoryFeed, batch)
	r.AddLabel("advisory_feed_schema", AdvisoryFeedContractVersion)
}

func (r *Result) WithAdvisoryFeed(batch AdvisoryFeedBatch) *Result {
	r.AddAdvisoryFeed(batch)
	return r
}
