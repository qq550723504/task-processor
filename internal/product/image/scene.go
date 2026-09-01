package image

import (
	"embed"
	"fmt"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type SceneOptions struct {
	SceneCategory     string
	SceneStyle        string
	BackgroundTone    string
	Composition       string
	PropsLevel        string
	AudienceHint      string
	CustomSceneHint   string
	SlotRole          string
	SlotBrief         string
	StyleReferenceIDs []string
}

type SceneColor struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

type SceneProfile struct {
	Name                 string
	Group                string
	BlurRadius           float64
	BackgroundBrightness float64
	BackgroundContrast   float64
	SubjectScale         float64
	CardColor            SceneColor
	BackgroundTemplate   string
	OverlayTemplate      string
	LayoutVariant        string
	VisualMode           string
	CopySlots            []string
	BadgeSlots           []string
	MeasurementSlots     []string
	DetailAnchorSlots    []string
	MaxCopyLines         int
	MaxBadges            int
	MeasurementMode      string
	DetailAnchorMode     string
}

const (
	MinSceneCanvasSize = 64
	MaxSceneCanvasSize = 8192
)

type SceneLayoutInput struct {
	CanvasSize    int
	SubjectWidth  int
	SubjectHeight int
}

type ScenePoint struct {
	X int
	Y int
}

type ScenePixelBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

type SceneLayout struct {
	CardWidth     int
	CardHeight    int
	CardPoint     ScenePoint
	SubjectPoint  ScenePoint
	SubjectBounds ScenePixelBounds
	CardOpacity   float64
	Engine        string
	QualityGrade  string
}

type ScenePlanRequest struct {
	ProfileName string
	Product     ProductContext
	Options     SceneOptions
	Geometry    SceneLayoutInput
}

type ScenePlan struct {
	ProfileName   string
	VisualMode    string
	LayoutVariant string
	Options       SceneOptions
	Content       []SceneContent
	Layers        []SceneLayer
	Layout        SceneLayout
}

type SceneContent struct {
	ID          string
	Kind        string
	Slot        string
	Text        string
	ContentType string
	SourceKey   string
	SourceType  string
	Priority    int
}

type SceneBounds struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type SceneLayer struct {
	ID          string
	Kind        string
	Region      string
	VisualRole  string
	Alignment   string
	StyleToken  string
	TextStyle   string
	Text        string
	RenderOrder int
	Bounds      SceneBounds
	Opacity     float64
}

type sceneProfileFile struct {
	Default  string             `yaml:"default"`
	Profiles []sceneProfileYAML `yaml:"presets"`
}

type sceneProfileYAML struct {
	Name                 string         `yaml:"name"`
	Group                string         `yaml:"group"`
	BlurRadius           float64        `yaml:"blur_radius"`
	BackgroundBrightness float64        `yaml:"background_brightness"`
	BackgroundContrast   float64        `yaml:"background_contrast"`
	SubjectScale         float64        `yaml:"subject_scale"`
	CardColor            sceneColorYAML `yaml:"card_color"`
	BackgroundTemplate   string         `yaml:"background_template"`
	OverlayTemplate      string         `yaml:"overlay_template"`
	LayoutVariant        string         `yaml:"layout_variant"`
	VisualMode           string         `yaml:"visual_mode"`
	CopySlots            []string       `yaml:"copy_slots"`
	BadgeSlots           []string       `yaml:"badge_slots"`
	MeasurementSlots     []string       `yaml:"measurement_slots"`
	DetailAnchorSlots    []string       `yaml:"detail_anchor_slots"`
	MaxCopyLines         int            `yaml:"max_copy_lines"`
	MaxBadges            int            `yaml:"max_badges"`
	MeasurementMode      string         `yaml:"measurement_mode"`
	DetailAnchorMode     string         `yaml:"detail_anchor_mode"`
}

type sceneColorYAML struct {
	R uint8 `yaml:"r"`
	G uint8 `yaml:"g"`
	B uint8 `yaml:"b"`
	A uint8 `yaml:"a"`
}

//go:embed presets/scene_profiles.yaml
var sceneProfileFiles embed.FS

func MergeSceneOptions(base, override *SceneOptions) (*SceneOptions, error) {
	if base == nil && override == nil {
		return nil, nil
	}
	merged := SceneOptions{}
	if base != nil {
		var err error
		merged, err = normalizedSceneOptions(*base)
		if err != nil {
			return nil, err
		}
	}
	if override != nil {
		normalized, err := normalizedSceneOptions(*override)
		if err != nil {
			return nil, err
		}
		mergeSceneOptionValue(&merged.SceneCategory, normalized.SceneCategory)
		mergeSceneOptionValue(&merged.SceneStyle, normalized.SceneStyle)
		mergeSceneOptionValue(&merged.BackgroundTone, normalized.BackgroundTone)
		mergeSceneOptionValue(&merged.Composition, normalized.Composition)
		mergeSceneOptionValue(&merged.PropsLevel, normalized.PropsLevel)
		mergeSceneOptionValue(&merged.AudienceHint, normalized.AudienceHint)
		mergeSceneOptionValue(&merged.CustomSceneHint, normalized.CustomSceneHint)
		mergeSceneOptionValue(&merged.SlotRole, normalized.SlotRole)
		mergeSceneOptionValue(&merged.SlotBrief, normalized.SlotBrief)
		if len(normalized.StyleReferenceIDs) > 0 {
			merged.StyleReferenceIDs = normalized.StyleReferenceIDs
		}
	}
	if err := validateSceneOptions(merged); err != nil {
		return nil, err
	}
	return &merged, nil
}

func ResolveSceneProfile(name string) (SceneProfile, error) {
	if len(name) > maxImageStringBytes {
		return SceneProfile{}, ErrInputInvalid
	}
	data, err := sceneProfileFiles.ReadFile("presets/scene_profiles.yaml")
	if err != nil {
		return SceneProfile{}, fmt.Errorf("%w: embedded scene profiles unavailable", ErrInputInvalid)
	}
	var document sceneProfileFile
	if err := yaml.Unmarshal(data, &document); err != nil {
		return SceneProfile{}, fmt.Errorf("%w: invalid embedded scene profiles", ErrInputInvalid)
	}
	requested := strings.TrimSpace(name)
	if requested == "" {
		requested = strings.TrimSpace(document.Default)
	}
	for _, raw := range document.Profiles {
		if strings.TrimSpace(raw.Name) != requested {
			continue
		}
		profile, err := normalizeSceneProfile(raw)
		if err != nil {
			return SceneProfile{}, err
		}
		return cloneSceneProfile(profile), nil
	}
	return SceneProfile{}, fmt.Errorf("%w: scene profile %q", ErrCapabilityUnsupported, requested)
}

func BuildScenePlan(request ScenePlanRequest) (ScenePlan, error) {
	product, err := validateProductContext(request.Product)
	if err != nil {
		return ScenePlan{}, err
	}
	if !isCanonicalOptional(request.ProfileName) {
		return ScenePlan{}, ErrInputInvalid
	}
	profile, err := ResolveSceneProfile(request.ProfileName)
	if err != nil {
		return ScenePlan{}, err
	}
	options, err := MergeSceneOptions(nil, &request.Options)
	if err != nil {
		return ScenePlan{}, err
	}
	if options == nil {
		options = &SceneOptions{}
	}
	if options.SceneCategory == "" {
		options.SceneCategory = inferSceneCategory(product)
	}
	layout, err := BuildSceneLayout(profile, request.Geometry)
	if err != nil {
		return ScenePlan{}, err
	}
	content := buildSceneContent(profile, product)
	layers := buildSceneLayers(profile, content, layout, request.Geometry.CanvasSize)
	return ScenePlan{
		ProfileName: profile.Name, VisualMode: profile.VisualMode, LayoutVariant: profile.LayoutVariant,
		Options: *options, Content: content, Layers: layers, Layout: layout,
	}, nil
}

func BuildSceneLayout(profile SceneProfile, input SceneLayoutInput) (SceneLayout, error) {
	if err := validateSceneLayoutInput(profile, input); err != nil {
		return SceneLayout{}, err
	}
	if profile.VisualMode == "selling_point" {
		return buildSellingPointSceneLayout(profile, input), nil
	}
	return buildPresetSceneLayout(profile, input), nil
}

func validateSceneLayoutInput(profile SceneProfile, input SceneLayoutInput) error {
	if input.CanvasSize < MinSceneCanvasSize || input.CanvasSize > MaxSceneCanvasSize ||
		input.SubjectWidth <= 0 || input.SubjectHeight <= 0 ||
		input.SubjectWidth > input.CanvasSize || input.SubjectHeight > input.CanvasSize ||
		profile.MaxCopyLines <= 0 || profile.MaxCopyLines > maxMetadataValues ||
		profile.MaxBadges <= 0 || profile.MaxBadges > maxMetadataValues {
		return ErrInputInvalid
	}
	return nil
}

func buildPresetSceneLayout(profile SceneProfile, input SceneLayoutInput) SceneLayout {
	canvasSize := input.CanvasSize
	baseReserve := maxSceneInt(canvasSize/20, 40)
	reserveTop := baseReserve
	reserveRight := baseReserve
	reserveBottom := baseReserve
	reserveLeft := baseReserve

	if profile.MaxBadges > 1 {
		reserveTop += canvasSize / 24
	}
	if profile.MaxCopyLines > 2 {
		reserveRight += canvasSize / 14
	}
	if profile.MeasurementMode == "dual_axis" {
		reserveBottom += canvasSize / 12
	}
	if profile.DetailAnchorMode == "dual_anchor" {
		reserveRight += canvasSize / 18
		reserveLeft += canvasSize / 36
	}

	switch profile.LayoutVariant {
	case "right_info_panel", "spec_sheet", "detail_grid":
		reserveRight += canvasSize / 10
	case "left_focus_panel":
		reserveLeft += canvasSize / 10
	case "hero_center", "editorial_full":
		reserveTop = maxSceneInt(reserveTop-canvasSize/36, baseReserve/2)
		reserveBottom = maxSceneInt(reserveBottom-canvasSize/36, baseReserve/2)
	}

	cardWidth := minSceneInt(canvasSize-(reserveLeft+reserveRight), input.SubjectWidth+reserveLeft+reserveRight)
	cardHeight := minSceneInt(canvasSize-(reserveTop+reserveBottom), input.SubjectHeight+reserveTop+reserveBottom)
	cardWidth = maxSceneInt(cardWidth, input.SubjectWidth+baseReserve)
	cardHeight = maxSceneInt(cardHeight, input.SubjectHeight+baseReserve)
	cardWidth = minSceneInt(cardWidth, canvasSize-baseReserve)
	cardHeight = minSceneInt(cardHeight, canvasSize-baseReserve)
	cardWidth = containSceneDimension(cardWidth, input.SubjectWidth, canvasSize)
	cardHeight = containSceneDimension(cardHeight, input.SubjectHeight, canvasSize)

	cardX := (canvasSize - cardWidth) / 2
	cardY := (canvasSize - cardHeight) / 2
	if reserveRight > reserveLeft {
		cardX -= (reserveRight - reserveLeft) / 2
	}
	if reserveBottom > reserveTop {
		cardY -= (reserveBottom - reserveTop) / 2
	}
	cardX = clampSceneInt(cardX, baseReserve/2, canvasSize-cardWidth-baseReserve/2)
	cardY = clampSceneInt(cardY, baseReserve/2, canvasSize-cardHeight-baseReserve/2)
	cardX = clampSceneInt(cardX, 0, canvasSize-cardWidth)
	cardY = clampSceneInt(cardY, 0, canvasSize-cardHeight)

	subjectX := cardX + reserveLeft + (cardWidth-reserveLeft-reserveRight-input.SubjectWidth)/2
	subjectY := cardY + reserveTop + (cardHeight-reserveTop-reserveBottom-input.SubjectHeight)/2
	subjectX = clampSceneSubject(subjectX, cardX, cardWidth, input.SubjectWidth, baseReserve/4, baseReserve/4)
	subjectY = clampSceneSubject(subjectY, cardY, cardHeight, input.SubjectHeight, baseReserve/4, baseReserve/4)

	opacity := 0.85
	switch profile.Group {
	case "editorial/model", "editorial":
		opacity = 0.82
	case "lifestyle/scene", "lifestyle":
		opacity = 0.88
	case "selling_point/size/spec/detail", "spec_detail":
		opacity = 0.91
	}
	return newSceneLayout(cardWidth, cardHeight, cardX, cardY, subjectX, subjectY, input, opacity, "preset_layout_v1")
}

func buildSellingPointSceneLayout(profile SceneProfile, input SceneLayoutInput) SceneLayout {
	canvasSize := input.CanvasSize
	baseReserve := maxSceneInt(canvasSize/18, 56)
	copyReserve := maxSceneInt(profile.MaxCopyLines, 1) * canvasSize / 18
	badgeReserve := maxSceneInt(profile.MaxBadges, 1) * canvasSize / 28

	leftReserve := baseReserve + canvasSize/16
	rightReserve := baseReserve + copyReserve
	topReserve := baseReserve + badgeReserve
	bottomReserve := baseReserve

	switch profile.MeasurementMode {
	case "dual_axis":
		bottomReserve += canvasSize / 10
	case "callout":
		bottomReserve += canvasSize / 14
	}
	switch profile.DetailAnchorMode {
	case "dual_anchor":
		leftReserve += canvasSize / 20
		rightReserve += canvasSize / 20
	case "side_stack":
		rightReserve += canvasSize / 16
	}
	switch profile.LayoutVariant {
	case "selling_point_grid":
		rightReserve += canvasSize / 18
	case "selling_point_stack":
		topReserve += canvasSize / 18
	case "selling_point_focus":
		leftReserve += canvasSize / 12
	}

	cardWidth := minSceneInt(canvasSize-(leftReserve+rightReserve), input.SubjectWidth+leftReserve+rightReserve)
	cardHeight := minSceneInt(canvasSize-(topReserve+bottomReserve), input.SubjectHeight+topReserve+bottomReserve)
	cardWidth = maxSceneInt(cardWidth, input.SubjectWidth+baseReserve+canvasSize/10)
	cardHeight = maxSceneInt(cardHeight, input.SubjectHeight+baseReserve+canvasSize/12)
	cardWidth = minSceneInt(cardWidth, canvasSize-baseReserve)
	cardHeight = minSceneInt(cardHeight, canvasSize-baseReserve)
	cardWidth = containSceneDimension(cardWidth, input.SubjectWidth, canvasSize)
	cardHeight = containSceneDimension(cardHeight, input.SubjectHeight, canvasSize)

	cardX := clampSceneInt(canvasSize/14, baseReserve/2, canvasSize-cardWidth-baseReserve/2)
	cardY := clampSceneInt((canvasSize-cardHeight)/2, baseReserve/2, canvasSize-cardHeight-baseReserve/2)
	cardX = clampSceneInt(cardX, 0, canvasSize-cardWidth)
	cardY = clampSceneInt(cardY, 0, canvasSize-cardHeight)

	subjectX := clampSceneSubject(cardX+leftReserve/2, cardX, cardWidth, input.SubjectWidth, baseReserve/3, baseReserve/2)
	subjectY := cardY + topReserve + (cardHeight-topReserve-bottomReserve-input.SubjectHeight)/2
	subjectY = clampSceneSubject(subjectY, cardY, cardHeight, input.SubjectHeight, baseReserve/3, baseReserve/2)

	return newSceneLayout(cardWidth, cardHeight, cardX, cardY, subjectX, subjectY, input, 0.93, "selling_point_layout_v1")
}

func newSceneLayout(cardWidth, cardHeight, cardX, cardY, subjectX, subjectY int, input SceneLayoutInput, opacity float64, engine string) SceneLayout {
	return SceneLayout{
		CardWidth: cardWidth, CardHeight: cardHeight, CardPoint: ScenePoint{X: cardX, Y: cardY},
		SubjectPoint:  ScenePoint{X: subjectX, Y: subjectY},
		SubjectBounds: ScenePixelBounds{X: subjectX, Y: subjectY, Width: input.SubjectWidth, Height: input.SubjectHeight},
		CardOpacity:   opacity, Engine: engine, QualityGrade: "ideal",
	}
}

func containSceneDimension(value, subjectSize, canvasSize int) int {
	value = maxSceneInt(value, subjectSize)
	return minSceneInt(value, canvasSize)
}

func clampSceneSubject(value, origin, size, subjectSize, leadingInset, trailingInset int) int {
	minimum := origin + leadingInset
	maximum := origin + size - subjectSize - trailingInset
	if maximum < minimum {
		minimum = origin
		maximum = origin + size - subjectSize
	}
	return clampSceneInt(value, minimum, maximum)
}

func clampSceneInt(value, minimum, maximum int) int {
	if maximum < minimum {
		return minimum
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func minSceneInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxSceneInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func normalizedSceneOptions(options SceneOptions) (SceneOptions, error) {
	if err := preflightSceneOptions(options); err != nil {
		return SceneOptions{}, err
	}
	options.SceneCategory = strings.TrimSpace(options.SceneCategory)
	options.SceneStyle = strings.TrimSpace(options.SceneStyle)
	options.BackgroundTone = strings.TrimSpace(options.BackgroundTone)
	options.Composition = strings.TrimSpace(options.Composition)
	options.PropsLevel = strings.TrimSpace(options.PropsLevel)
	options.AudienceHint = strings.TrimSpace(options.AudienceHint)
	options.CustomSceneHint = strings.TrimSpace(options.CustomSceneHint)
	options.SlotRole = strings.TrimSpace(options.SlotRole)
	options.SlotBrief = strings.TrimSpace(options.SlotBrief)
	styleReferenceIDs, err := normalizedStrings(options.StyleReferenceIDs, maxStyleReferences)
	if err != nil {
		return SceneOptions{}, err
	}
	options.StyleReferenceIDs = styleReferenceIDs
	return options, nil
}

func preflightSceneOptions(options SceneOptions) error {
	if len(options.StyleReferenceIDs) > maxStyleReferences {
		return ErrInputInvalid
	}
	used := 0
	values := [...]string{
		options.SceneCategory, options.SceneStyle, options.BackgroundTone, options.Composition,
		options.PropsLevel, options.AudienceHint, options.CustomSceneHint, options.SlotRole, options.SlotBrief,
	}
	for _, value := range values {
		if !addImageStringBytes(&used, value) {
			return ErrInputInvalid
		}
	}
	for _, reference := range options.StyleReferenceIDs {
		if !addImageStringBytes(&used, reference) {
			return ErrInputInvalid
		}
	}
	return nil
}

func validateSceneOptions(options SceneOptions) error {
	return preflightSceneOptions(options)
}

func mergeSceneOptionValue(target *string, value string) {
	if value != "" {
		*target = value
	}
}

func normalizeSceneProfile(raw sceneProfileYAML) (SceneProfile, error) {
	for _, value := range []float64{raw.BlurRadius, raw.BackgroundBrightness, raw.BackgroundContrast, raw.SubjectScale} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return SceneProfile{}, ErrInputInvalid
		}
	}
	copySlots, err := normalizedProfileSlots(raw.CopySlots)
	if err != nil {
		return SceneProfile{}, err
	}
	badgeSlots, err := normalizedProfileSlots(raw.BadgeSlots)
	if err != nil {
		return SceneProfile{}, err
	}
	measurementSlots, err := normalizedProfileSlots(raw.MeasurementSlots)
	if err != nil {
		return SceneProfile{}, err
	}
	detailAnchorSlots, err := normalizedProfileSlots(raw.DetailAnchorSlots)
	if err != nil {
		return SceneProfile{}, err
	}
	profile := SceneProfile{
		Name: strings.TrimSpace(raw.Name), Group: strings.TrimSpace(raw.Group),
		BlurRadius: raw.BlurRadius, BackgroundBrightness: raw.BackgroundBrightness,
		BackgroundContrast: raw.BackgroundContrast, SubjectScale: raw.SubjectScale,
		CardColor:          SceneColor{R: raw.CardColor.R, G: raw.CardColor.G, B: raw.CardColor.B, A: raw.CardColor.A},
		BackgroundTemplate: strings.TrimSpace(raw.BackgroundTemplate), OverlayTemplate: strings.TrimSpace(raw.OverlayTemplate),
		LayoutVariant: strings.TrimSpace(raw.LayoutVariant), VisualMode: strings.TrimSpace(raw.VisualMode),
		CopySlots: copySlots, BadgeSlots: badgeSlots,
		MeasurementSlots: measurementSlots, DetailAnchorSlots: detailAnchorSlots,
		MaxCopyLines: raw.MaxCopyLines, MaxBadges: raw.MaxBadges,
		MeasurementMode: strings.TrimSpace(raw.MeasurementMode), DetailAnchorMode: strings.TrimSpace(raw.DetailAnchorMode),
	}
	if !isCanonicalRequired(profile.Name) || profile.SubjectScale <= 0 || profile.SubjectScale > 1 || profile.LayoutVariant == "" || profile.VisualMode == "" {
		return SceneProfile{}, ErrInputInvalid
	}
	if profile.MaxCopyLines <= 0 {
		profile.MaxCopyLines = 2
	}
	if profile.MaxBadges <= 0 {
		profile.MaxBadges = 1
	}
	if profile.MeasurementMode == "" {
		profile.MeasurementMode = "single_axis"
	}
	if profile.DetailAnchorMode == "" {
		profile.DetailAnchorMode = "single_anchor"
	}
	return profile, nil
}

func cloneSceneProfile(profile SceneProfile) SceneProfile {
	profile.CopySlots = append([]string(nil), profile.CopySlots...)
	profile.BadgeSlots = append([]string(nil), profile.BadgeSlots...)
	profile.MeasurementSlots = append([]string(nil), profile.MeasurementSlots...)
	profile.DetailAnchorSlots = append([]string(nil), profile.DetailAnchorSlots...)
	return profile
}

func normalizedProfileSlots(values []string) ([]string, error) {
	return normalizedStrings(values, maxMetadataValues)
}

type sceneContentCandidate struct {
	Text        string
	ContentType string
	SourceKey   string
	SourceType  string
}

func buildSceneContent(profile SceneProfile, product ProductContext) []SceneContent {
	attributes := sortedSceneAttributes(product)
	measurements, details, badges := classifySceneAttributes(attributes)
	copyCandidates := []sceneContentCandidate{{Text: product.Title, ContentType: "headline", SourceKey: "title", SourceType: "product_context"}}
	if product.ProductType != "" && !strings.EqualFold(product.ProductType, product.Title) {
		copyCandidates = append(copyCandidates, sceneContentCandidate{Text: product.ProductType, ContentType: "supporting_copy", SourceKey: "product_type", SourceType: "product_context"})
	}
	copyCandidates = append(copyCandidates, details...)
	if len(badges) == 0 && product.ProductType != "" {
		badges = append(badges, sceneContentCandidate{Text: product.ProductType, ContentType: "badge", SourceKey: "product_type", SourceType: "product_context"})
	}
	content := make([]SceneContent, 0)
	content = appendAssignedSceneContent(content, "copy", profile.CopySlots, uniqueSceneCandidates(copyCandidates), profile.MaxCopyLines)
	content = appendAssignedSceneContent(content, "badge", profile.BadgeSlots, badges, profile.MaxBadges)
	content = appendAssignedSceneContent(content, "measurement", profile.MeasurementSlots, measurements, len(profile.MeasurementSlots))
	content = appendAssignedSceneContent(content, "detail_anchor", profile.DetailAnchorSlots, details, len(profile.DetailAnchorSlots))
	return content
}

func sortedSceneAttributes(product ProductContext) []sceneContentCandidate {
	keys := make([]string, 0, len(product.Attributes))
	for key := range product.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]sceneContentCandidate, 0, len(keys))
	for _, key := range keys {
		result = append(result, sceneContentCandidate{
			Text: key + ": " + product.Attributes[key], SourceKey: key, SourceType: "attribute",
		})
	}
	return result
}

func classifySceneAttributes(values []sceneContentCandidate) (measurements, details, badges []sceneContentCandidate) {
	for _, value := range values {
		key := strings.ToLower(value.SourceKey)
		switch {
		case containsAnyFold(key, "size", "dimension", "length", "width", "height", "weight"):
			value.ContentType = "measurement"
			measurements = append(measurements, value)
		case containsAnyFold(key, "material", "fabric", "feature", "style", "fit"):
			value.ContentType = "badge"
			badges = append(badges, value)
		default:
			value.ContentType = "detail_anchor"
			details = append(details, value)
		}
	}
	return uniqueSceneCandidates(measurements), uniqueSceneCandidates(details), uniqueSceneCandidates(badges)
}

func uniqueSceneCandidates(values []sceneContentCandidate) []sceneContentCandidate {
	seen := make(map[string]struct{}, len(values))
	result := make([]sceneContentCandidate, 0, len(values))
	for _, value := range values {
		value.Text = strings.TrimSpace(value.Text)
		if value.Text == "" {
			continue
		}
		identity := strings.ToLower(value.Text + "|" + value.SourceKey + "|" + value.ContentType)
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, value)
	}
	return result
}

func appendAssignedSceneContent(destination []SceneContent, kind string, slots []string, values []sceneContentCandidate, limit int) []SceneContent {
	if limit <= 0 || limit > len(values) {
		limit = len(values)
	}
	if limit > len(slots) {
		limit = len(slots)
	}
	for index := 0; index < limit; index++ {
		value := values[index]
		destination = append(destination, SceneContent{
			ID: kind + ":" + slots[index], Kind: kind, Slot: slots[index], Text: value.Text,
			ContentType: value.ContentType, SourceKey: value.SourceKey, SourceType: value.SourceType, Priority: index + 1,
		})
	}
	return destination
}

func buildSceneLayers(profile SceneProfile, content []SceneContent, layout SceneLayout, canvasSize int) []SceneLayer {
	layers := []SceneLayer{
		{ID: "background", Kind: "background", Region: "full_canvas", VisualRole: "background", Alignment: "center", StyleToken: "background:" + profile.BackgroundTemplate, RenderOrder: 1, Bounds: sceneLayerBounds(profile.LayoutVariant, "full_canvas"), Opacity: 1},
		{ID: "card", Kind: "card", Region: "content_frame", VisualRole: "card", Alignment: "center", StyleToken: "overlay:" + profile.OverlayTemplate, RenderOrder: 2, Bounds: normalizedSceneBounds(layout.CardPoint.X, layout.CardPoint.Y, layout.CardWidth, layout.CardHeight, canvasSize), Opacity: layout.CardOpacity},
		{ID: "subject", Kind: "subject", Region: "product_focus", VisualRole: "subject", Alignment: "center-left", StyleToken: "subject:" + profile.LayoutVariant, RenderOrder: 3, Bounds: normalizedSceneBounds(layout.SubjectBounds.X, layout.SubjectBounds.Y, layout.SubjectBounds.Width, layout.SubjectBounds.Height, canvasSize), Opacity: 1},
	}
	for index, item := range content {
		region := sceneRegionForContent(profile.LayoutVariant, item.Kind)
		layers = append(layers, SceneLayer{
			ID: "content:" + item.ID, Kind: sceneLayerKind(item.Kind), Region: region,
			VisualRole: sceneVisualRole(item.Kind), Alignment: sceneLayerAlignment(region, item.Kind, item.ContentType),
			StyleToken: sceneStyleToken(profile, item.Kind, item.ContentType), TextStyle: sceneTextStyle(item.ContentType),
			Text: item.Text, RenderOrder: 100 + index + 1, Bounds: sceneLayerBounds(profile.LayoutVariant, region), Opacity: 1,
		})
	}
	return layers
}

func normalizedSceneBounds(x, y, width, height, canvasSize int) SceneBounds {
	canvas := float64(canvasSize)
	return SceneBounds{
		X: float64(x) / canvas, Y: float64(y) / canvas,
		Width: float64(width) / canvas, Height: float64(height) / canvas,
	}
}

func sceneRegionForContent(layoutVariant, kind string) string {
	switch kind {
	case "badge":
		return "top_band"
	case "copy":
		if layoutVariant == "selling_point_stack" || layoutVariant == "selling_point_focus" {
			return "right_panel"
		}
		return "headline_panel"
	case "measurement":
		return "bottom_band"
	case "detail_anchor":
		return "side_panel"
	default:
		return "content_panel"
	}
}

func sceneLayerBounds(layoutVariant, region string) SceneBounds {
	switch region {
	case "full_canvas":
		return SceneBounds{Width: 1, Height: 1}
	case "content_frame":
		return SceneBounds{X: 0.08, Y: 0.08, Width: 0.84, Height: 0.84}
	case "product_focus":
		if layoutVariant == "info_card_right" || layoutVariant == "selling_point_focus" {
			return SceneBounds{X: 0.10, Y: 0.16, Width: 0.42, Height: 0.66}
		}
		return SceneBounds{X: 0.14, Y: 0.18, Width: 0.48, Height: 0.60}
	case "top_band":
		return SceneBounds{X: 0.12, Y: 0.08, Width: 0.72, Height: 0.10}
	case "headline_panel":
		return SceneBounds{X: 0.57, Y: 0.20, Width: 0.25, Height: 0.22}
	case "right_panel":
		return SceneBounds{X: 0.56, Y: 0.20, Width: 0.26, Height: 0.34}
	case "bottom_band":
		return SceneBounds{X: 0.18, Y: 0.78, Width: 0.60, Height: 0.10}
	case "side_panel":
		return SceneBounds{X: 0.82, Y: 0.22, Width: 0.10, Height: 0.40}
	default:
		return SceneBounds{X: 0.14, Y: 0.14, Width: 0.72, Height: 0.72}
	}
}

func sceneLayerAlignment(region, kind, contentType string) string {
	switch region {
	case "top_band":
		return "top-left"
	case "bottom_band":
		return "bottom-center"
	case "side_panel":
		return "right-center"
	}
	if kind == "copy" && contentType == "headline" {
		return "top-left"
	}
	return "left-stack"
}

func sceneLayerKind(kind string) string {
	switch kind {
	case "copy":
		return "text"
	case "measurement":
		return "spec"
	case "detail_anchor":
		return "detail"
	default:
		return kind
	}
}

func sceneVisualRole(kind string) string {
	switch kind {
	case "badge":
		return "attention"
	case "copy":
		return "message"
	case "measurement":
		return "specification"
	case "detail_anchor":
		return "detail_callout"
	default:
		return "content"
	}
}

func sceneStyleToken(profile SceneProfile, kind, contentType string) string {
	switch kind {
	case "badge":
		return "badge:" + profile.VisualMode
	case "measurement":
		return "spec:" + profile.MeasurementMode
	case "detail_anchor":
		return "detail:" + profile.DetailAnchorMode
	case "copy":
		if contentType == "headline" {
			return "text:headline"
		}
		return "text:supporting"
	default:
		return "content:default"
	}
}

func sceneTextStyle(contentType string) string {
	switch contentType {
	case "headline":
		return "headline-strong"
	case "supporting_copy":
		return "body-compact"
	default:
		return ""
	}
}

func inferSceneCategory(product ProductContext) string {
	candidates := []string{product.Attributes["scene_category"], product.Attributes["category"], product.Attributes["product_category"], product.ProductType, product.Title}
	for _, candidate := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		switch {
		case containsAnyFold(normalized, "sneaker", "shoe", "boot", "sandal", "slipper", "heel", "loafer"):
			return "shoes"
		case containsAnyFold(normalized, "necklace", "ring", "earring", "bracelet", "jewelry", "jewellery", "pendant", "brooch"):
			return "jewelry"
		case containsAnyFold(normalized, "handbag", "backpack", "bag", "purse", "tote", "satchel", "crossbody"):
			return "bags"
		}
	}
	return ""
}
