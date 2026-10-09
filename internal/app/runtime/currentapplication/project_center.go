package currentapplication

import "errors"

type ProjectCenterConfig struct {
	Database DatabaseConfig `json:"database"`
}

func (c *Config) validateProjectCenter() error {
	if c.ProjectCenter == nil {
		return nil
	}
	d := c.ProjectCenter.Database
	if e := d.validate("projectCenter.database"); e != nil {
		return e
	}
	if d.User != "ai_projects_runtime" || d.MaxConnections > 4 {
		return errors.New("project center requires its restricted independent owner")
	}
	return nil
}
