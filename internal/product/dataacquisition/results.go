package dataacquisition

import (
	"context"
	"encoding/json"
	"task-processor/internal/product/collection"
)

type Usage struct {
	DayRows           int64    `json:"dayRows"`
	MonthConfirmedFen int64    `json:"monthConfirmedFen"`
	MonthPendingFen   int64    `json:"monthPendingFen"`
	FinishedJobs      int64    `json:"finishedJobs"`
	SucceededJobs     int64    `json:"succeededJobs"`
	SuccessRate       *float64 `json:"successRate"`
	Window            string   `json:"window"`
}
type KeyQuota struct {
	KeyID            string `json:"keyId"`
	DayConsumedRows  int64  `json:"dayConsumedRows"`
	DayReservedRows  int64  `json:"dayReservedRows"`
	MonthConsumedFen int64  `json:"monthConsumedFen"`
	MonthReservedFen int64  `json:"monthReservedFen"`
}
type Result struct {
	ID      string             `json:"id"`
	State   string             `json:"state"`
	Reason  string             `json:"reason,omitempty"`
	Source  *collection.Source `json:"source,omitempty"`
	Data    map[string]any     `json:"data,omitempty"`
	Missing []string           `json:"missing,omitempty"`
}
type ResultPage struct {
	Items      []Result `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
}
type CapturedResultReader interface {
	Verify(context.Context, collection.Scope, collection.Source, Evidence) error
}

func (s *Service) Results(ctx context.Context, p Principal, id, cursor string, limit int, reader CapturedResultReader) (ResultPage, error) {
	if reader == nil || p.Scope.Validate() != nil || !collection.ValidID(id) || (cursor != "" && !collection.ValidID(cursor)) || limit < 1 || limit > 100 {
		return ResultPage{}, ErrInvalid
	}
	if p.CredentialID != "" {
		var page ResultPage
		err := s.repo.WithResultRead(ctx, p, func(ctx context.Context, repo ResultReadRepository, reader CapturedResultReader) error {
			var err error
			page, err = s.results(ctx, p, id, cursor, limit, repo, reader)
			return err
		})
		if err != nil {
			return ResultPage{}, err
		}
		return page, nil
	}
	return s.results(ctx, p, id, cursor, limit, s.repo, reader)
}
func (s *Service) results(ctx context.Context, p Principal, id, cursor string, limit int, repo ResultReadRepository, reader CapturedResultReader) (ResultPage, error) {
	job, err := s.read(ctx, p, id, repo)
	if err != nil {
		return ResultPage{}, err
	}
	items, err := repo.Items(ctx, job)
	if err != nil {
		return ResultPage{}, err
	}
	page := ResultPage{Items: []Result{}}
	for _, item := range items {
		if cursor != "" && item.ID <= cursor {
			continue
		}
		if len(page.Items) == limit {
			page.NextCursor = page.Items[len(page.Items)-1].ID
			break
		}
		result := Result{ID: item.ID, State: item.State, Reason: item.Reason, Source: item.Source}
		if item.State == "SAVED" {
			if item.Source == nil || item.Evidence == nil {
				return ResultPage{}, ErrUnavailable
			}
			if err = reader.Verify(ctx, job.Scope, *item.Source, *item.Evidence); err != nil {
				return ResultPage{}, err
			}
			e := item.Evidence
			raw, _ := json.Marshal(e)
			var data map[string]any
			if json.Unmarshal(raw, &data) != nil {
				return ResultPage{}, ErrUnavailable
			}
			site, _ := ResolveSite(e.Site)
			data["sourceUrl"] = "https://" + site.Domain + "/dp/" + e.ASIN
			result.Data = map[string]any{}
			for _, field := range job.Query.Fields {
				if value, ok := data[field]; ok {
					result.Data[field] = value
				}
			}
			if len(e.Images) == 0 {
				if includes(job.Query.Fields, "images") {
					result.Data["images"] = []string{e.MainImage}
				}
			}
			for _, field := range e.Missing {
				if includes(job.Query.Fields, field) {
					result.Missing = append(result.Missing, field)
				}
			}
		}
		candidate := ResultPage{Items: append(append([]Result(nil), page.Items...), result)}
		raw, err := json.Marshal(candidate)
		if err != nil {
			return ResultPage{}, ErrUnavailable
		}
		if len(raw) > (2<<20)-1024 {
			if len(page.Items) == 0 {
				return ResultPage{}, ErrUnavailable
			}
			page.NextCursor = page.Items[len(page.Items)-1].ID
			break
		}
		page.Items = append(page.Items, result)
	}
	return page, nil
}
func includes(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
