package platformsection

type Builder struct {
	Platform string
	Build    func() error
}

func BuildAll(builders []Builder) error {
	for _, builder := range builders {
		if builder.Build == nil {
			continue
		}
		if err := builder.Build(); err != nil {
			return err
		}
	}
	return nil
}
