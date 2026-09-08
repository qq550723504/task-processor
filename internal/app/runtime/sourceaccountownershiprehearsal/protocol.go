package sourceaccountownershiprehearsal

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"task-processor/internal/integration/persistence/sourceaccount/ownershipmigration"
)

const maxProtocolFrameBytes = 1024 * 1024

const (
	stageInspect       = "inspect"
	stageInstallSchema = "install-schema"
	stagePrepare       = "prepare"
	stageReceiptRead   = "receipt-read"
)

type databaseIdentity struct {
	SystemIdentifier string `json:"system_identifier"`
	Database         string `json:"database"`
	OID              uint32 `json:"oid"`
}

type stageChallenge struct {
	Stage     string `json:"stage"`
	Nonce     string `json:"nonce"`
	ChildPID  int    `json:"child_pid"`
	ParentPID int    `json:"parent_pid"`
}

type stageGrant struct {
	Invocation             string                     `json:"invocation"`
	Stage                  string                     `json:"stage"`
	Nonce                  string                     `json:"nonce"`
	ChildPID               int                        `json:"child_pid"`
	ParentPID              int                        `json:"parent_pid"`
	Container              string                     `json:"container_id"`
	Source                 databaseIdentity           `json:"source"`
	Metadata               databaseIdentity           `json:"metadata"`
	SourceDSN              string                     `json:"source_dsn"`
	MetadataDSN            string                     `json:"metadata_dsn"`
	SourceID               string                     `json:"source_id"`
	MetadataID             string                     `json:"metadata_id"`
	ProfileRoot            string                     `json:"profile_root"`
	ReceiptPath            string                     `json:"receipt_path,omitempty"`
	IdempotencyKey         string                     `json:"idempotency_key,omitempty"`
	Preflight              ownershipmigration.Receipt `json:"preflight,omitempty"`
	DropResponseAfterWrite bool                       `json:"drop_response_after_write,omitempty"`
}

type stageResult struct {
	Invocation string                              `json:"invocation"`
	Stage      string                              `json:"stage"`
	Nonce      string                              `json:"nonce"`
	Status     string                              `json:"status"`
	Code       int                                 `json:"code"`
	Digest     string                              `json:"digest,omitempty"`
	Count      int                                 `json:"count,omitempty"`
	Replayed   bool                                `json:"replayed,omitempty"`
	Preflight  *ownershipmigration.Receipt         `json:"preflight,omitempty"`
	Prepared   *ownershipmigration.PreparedReceipt `json:"prepared,omitempty"`
}

func decodeFrame(reader io.Reader, destination any) error {
	buffered, ok := reader.(*bufio.Reader)
	if !ok {
		buffered = bufio.NewReaderSize(reader, maxProtocolFrameBytes+2)
	}
	frame, err := buffered.ReadBytes('\n')
	if len(frame) > maxProtocolFrameBytes {
		return errors.New("protocol frame exceeds byte limit")
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read protocol frame: %w", err)
	}
	if len(frame) == 0 {
		return errors.New("protocol frame is empty")
	}
	frame = bytesTrimSpace(frame)
	decoder := json.NewDecoder(strings.NewReader(string(frame)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode protocol frame: %w", err)
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("protocol frame contains trailing data")
	}
	return nil
}

func encodeFrame(writer io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode protocol frame: %w", err)
	}
	if len(payload)+1 > maxProtocolFrameBytes {
		return errors.New("protocol frame exceeds byte limit")
	}
	payload = append(payload, '\n')
	if _, err = writer.Write(payload); err != nil {
		return fmt.Errorf("write protocol frame: %w", err)
	}
	return nil
}

func validateGrant(challenge stageChallenge, grant stageGrant) error {
	if !validStage(challenge.Stage) || grant.Stage != challenge.Stage || grant.Nonce != challenge.Nonce ||
		grant.ChildPID != challenge.ChildPID || grant.ParentPID != challenge.ParentPID {
		return errors.New("parent grant does not match one-shot child challenge")
	}
	if !boundedToken(grant.Invocation, 128) || !boundedToken(grant.Container, 128) {
		return errors.New("parent grant has invalid allocation identity")
	}
	if !validDatabaseIdentity(grant.Source) || !validDatabaseIdentity(grant.Metadata) ||
		grant.Source.SystemIdentifier != grant.Metadata.SystemIdentifier || grant.Source.OID == grant.Metadata.OID {
		return errors.New("parent grant has invalid database identity")
	}
	if grant.Source.Database != "source_b2" || grant.Metadata.Database != "metadata_b2" ||
		grant.SourceID != "issue364/rehearsal/source" || grant.MetadataID != "issue364/rehearsal/metadata" {
		return errors.New("parent grant is not the fixed rehearsal fixture")
	}
	if !filepath.IsAbs(grant.ProfileRoot) || !filepath.IsAbs(grant.ReceiptPath) {
		return errors.New("parent grant has invalid rehearsal paths")
	}
	if err := validateLoopbackDSN(grant.SourceDSN, grant.Source.Database); err != nil {
		return err
	}
	expectedRole := map[string]string{
		stageInspect:       "b2_inspect",
		stageInstallSchema: "b2_schema",
		stagePrepare:       "b2_prepare",
		stageReceiptRead:   "b2_receipt",
	}[challenge.Stage]
	if dsnUsername(grant.SourceDSN) != expectedRole {
		return errors.New("parent grant has the wrong fixed source role")
	}
	sourcePassword := dsnPassword(grant.SourceDSN)
	if sourcePassword == "" {
		return errors.New("parent grant source credential is missing")
	}
	if challenge.Stage != stageInstallSchema {
		if err := validateLoopbackDSN(grant.MetadataDSN, grant.Metadata.Database); err != nil {
			return err
		}
		if dsnUsername(grant.MetadataDSN) != expectedRole || dsnPassword(grant.MetadataDSN) != sourcePassword ||
			!sameDSNEndpoint(grant.SourceDSN, grant.MetadataDSN) {
			return errors.New("parent grant has the wrong fixed metadata role or cluster endpoint")
		}
	}
	if challenge.Stage == stagePrepare || challenge.Stage == stageReceiptRead {
		if !boundedToken(grant.IdempotencyKey, 256) || !validPinnedReceipt(grant.Preflight, grant) {
			return errors.New("parent grant has invalid pinned preflight evidence")
		}
	} else if grant.IdempotencyKey != "" || grant.Preflight.Version != 0 || grant.DropResponseAfterWrite {
		return errors.New("parent grant contains fields not allowed for this stage")
	}
	if challenge.Stage != stagePrepare && grant.DropResponseAfterWrite {
		return errors.New("response-loss injection is prepare-only")
	}
	return nil
}

func validateLoopbackDSN(value, database string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.User == nil {
		return errors.New("parent grant has invalid PostgreSQL connection")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || parsed.Port() == "" || strings.TrimPrefix(parsed.EscapedPath(), "/") != url.PathEscape(database) {
		return errors.New("parent grant PostgreSQL target is not the admitted loopback database")
	}
	query := parsed.Query()
	if len(query) != 1 || query.Get("sslmode") != "disable" || len(query["sslmode"]) != 1 || parsed.Fragment != "" {
		return errors.New("parent grant PostgreSQL options are not fixed")
	}
	return nil
}

func dsnUsername(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return ""
	}
	return parsed.User.Username()
}

func dsnPassword(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return ""
	}
	password, _ := parsed.User.Password()
	return password
}

func sameDSNEndpoint(left, right string) bool {
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	return leftErr == nil && rightErr == nil && leftURL.Host == rightURL.Host
}

func validPinnedReceipt(receipt ownershipmigration.Receipt, grant stageGrant) bool {
	if receipt.Version != 1 || receipt.Stage != "preflight_only" || receipt.SnapshotConsistency != "separate_non_atomic_snapshots" ||
		len(receipt.Digest) != 64 || receipt.AccountObservation.SourceID != grant.SourceID ||
		receipt.AccountObservation.Database != grant.Source.Database || receipt.MetadataObservation.SourceID != grant.MetadataID ||
		receipt.MetadataObservation.Database != grant.Metadata.Database {
		return false
	}
	for _, value := range receipt.Digest {
		if !strings.ContainsRune("0123456789abcdef", value) {
			return false
		}
	}
	return true
}

func validDatabaseIdentity(identity databaseIdentity) bool {
	return boundedToken(identity.SystemIdentifier, 64) && boundedToken(identity.Database, 63) && identity.OID != 0
}

func boundedToken(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
}

func validStage(stage string) bool {
	switch stage {
	case stageInspect, stageInstallSchema, stagePrepare, stageReceiptRead:
		return true
	default:
		return false
	}
}

func newNonce() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate rehearsal nonce: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func classifyPrepareError(err error) int {
	if errors.Is(err, ownershipmigration.ErrIdempotencyConflict) ||
		errors.Is(err, ownershipmigration.ErrSourceDrift) ||
		errors.Is(err, ownershipmigration.ErrTargetConflict) ||
		errors.Is(err, ownershipmigration.ErrTargetDrift) {
		return ExitConflictOrDrift
	}
	return ExitOutcomeUnknown
}

func bytesTrimSpace(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}
