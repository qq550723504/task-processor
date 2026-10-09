package storecenterapp

import (
	"sort"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/storecenter"
)

type OfficialApplicationRegistration struct {
	Provider   storecenter.OfficialConnectionProvider
	Protection storecenter.OfficialCredentialProtection
	Type       storecenter.OfficialApplicationType
}
type officialApplicationEntry struct {
	application storecenter.OfficialApplication
	provider    storecenter.OfficialConnectionProvider
	protection  storecenter.OfficialCredentialProtection
	mode        storecenter.OfficialApplicationType
}
type OfficialApplicationChoice = storecenter.OfficialApplicationChoice
type OfficialApplicationRegistry struct {
	entries map[string]officialApplicationEntry
}

// Including mode in the persisted revision makes a configuration type change
// invalidate every previously connected attempt instead of changing its rules.
func BoundOfficialRevision(version string, mode storecenter.OfficialApplicationType) string {
	return version + ":" + string(mode)
}
func NewOfficialApplicationRegistry(registrations []OfficialApplicationRegistration) (*OfficialApplicationRegistry, error) {
	if len(registrations) < 1 || len(registrations) > 16 {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	r := &OfficialApplicationRegistry{entries: map[string]officialApplicationEntry{}}
	for _, registration := range registrations {
		if registration.Provider == nil || registration.Protection == nil || !registration.Type.Valid() {
			return nil, storecenter.ErrOfficialConnectionUnavailable
		}
		app := registration.Provider.Application()
		base, ok := strings.CutSuffix(app.Version, ":"+string(registration.Type))
		if !ok || !authidentity.IsBoundedIdentifier(base) || !authidentity.IsBoundedIdentifier(app.Version) || !authidentity.IsBoundedIdentifier(app.AppID) {
			return nil, storecenter.ErrOfficialConnectionUnavailable
		}
		if _, duplicate := r.entries[app.AppID]; duplicate {
			return nil, storecenter.ErrOfficialConnectionUnavailable
		}
		r.entries[app.AppID] = officialApplicationEntry{application: app, provider: registration.Provider, protection: registration.Protection, mode: registration.Type}
	}
	return r, nil
}
func (r *OfficialApplicationRegistry) resolve(id, revision string) (officialApplicationEntry, error) {
	if r == nil {
		return officialApplicationEntry{}, storecenter.ErrOfficialConnectionUnavailable
	}
	entry, ok := r.entries[id]
	if !ok || entry.application.Version != revision || entry.provider.Application() != entry.application {
		return officialApplicationEntry{}, storecenter.ErrOfficialConnectionUnavailable
	}
	return entry, nil
}
func (r *OfficialApplicationRegistry) selectApplication(id string) (officialApplicationEntry, error) {
	if r == nil {
		return officialApplicationEntry{}, storecenter.ErrOfficialConnectionUnavailable
	}
	entry, ok := r.entries[id]
	if !ok {
		return officialApplicationEntry{}, storecenter.ErrOfficialConnectionUnavailable
	}
	return r.resolve(id, entry.application.Version)
}
func (r *OfficialApplicationRegistry) Applications() []OfficialApplicationChoice {
	choices := []OfficialApplicationChoice{}
	if r == nil {
		return choices
	}
	for _, entry := range r.entries {
		if _, err := r.resolve(entry.application.AppID, entry.application.Version); err == nil {
			choices = append(choices, OfficialApplicationChoice{AppID: entry.application.AppID, Revision: entry.application.Version, Type: entry.mode})
		}
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].AppID < choices[j].AppID })
	return choices
}
