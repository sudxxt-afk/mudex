package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"tui-player/internal/cover"
)

// VizStyle selects the visualizer rendering style.
type VizStyle int

const (
	VizBars VizStyle = iota
	VizVU
	VizWave
)

// Theme describes UI colors, cover grading, visualizer style and borders.
type Theme struct {
	ID            string
	Title         string
	Text          string
	Dim           string
	Accent        string
	Accent2       string
	Peak          string
	SelBG         string
	BorderFG      string
	Grade         cover.Grade
	Viz           VizStyle
	RoundedBorder bool
}

// AllThemes returns all 6 themes in fixed order.
// Palettes follow proven schemes (Gruvbox, Tokyo Night, neutrals) so that
// contrast holds on the theme background; see TestThemeContrast.
func AllThemes() []Theme {
	return []Theme{
		{
			ID: "neon", Title: "#00E5FF", Text: "#E8E8F0", Dim: "#8B8FA3",
			Accent: "#F471B8", Accent2: "#00E5A0", Peak: "#F471B8",
			SelBG: "#1a1a2e", BorderFG: "#6D5BD0",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: true,
		},
		{
			// Gruvbox dark
			ID: "lamp", Title: "#FABD2F", Text: "#EBDBB2", Dim: "#928374",
			Accent: "#FABD2F", Accent2: "#FE8019", Peak: "#FB4934",
			SelBG: "#3C3836", BorderFG: "#665C54",
			Grade: cover.GradeSepia, Viz: VizVU, RoundedBorder: false,
		},
		{
			ID: "minimal", Title: "#E5E5E5", Text: "#D4D4D4", Dim: "#737373",
			Accent: "#FAFAFA", Accent2: "#A3A3A3", Peak: "#FFFFFF",
			SelBG: "#1F1F1F", BorderFG: "#404040",
			Grade: cover.GradeMono, Viz: VizWave, RoundedBorder: false,
		},
		{
			ID: "phosphor", Title: "#5CFF5C", Text: "#3FD83F", Dim: "#1E7A1E",
			Accent: "#5CFF5C", Accent2: "#2EA82E", Peak: "#E8FF66",
			SelBG: "#0D1A0D", BorderFG: "#245C24",
			Grade: cover.GradePhosphor, Viz: VizBars, RoundedBorder: true,
		},
		{
			// Gruvbox light
			ID: "paper", Title: "#3C3836", Text: "#3C3836", Dim: "#928374",
			Accent: "#AF3A03", Accent2: "#79740E", Peak: "#9D0006",
			SelBG: "#EBDDB2", BorderFG: "#D5C4A1",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: false,
		},
		{
			// Tokyo Night
			ID: "midnight", Title: "#7DCFFF", Text: "#C0CAF5", Dim: "#565F89",
			Accent: "#7AA2F7", Accent2: "#7DCFFF", Peak: "#F7768E",
			SelBG: "#24283B", BorderFG: "#3B4261",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: true,
		},
	}
}

// ThemeByID returns the theme with the given id, or neon for unknown ids.
func ThemeByID(id string) Theme {
	for _, th := range AllThemes() {
		if th.ID == id {
			return th
		}
	}
	return AllThemes()[0]
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mudex", "config.json"), nil
}

// LoadTheme reads the saved theme id, falling back to neon on any error.
func LoadTheme() Theme {
	p, err := configPath()
	if err != nil {
		return ThemeByID("neon")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ThemeByID("neon")
	}
	var cfg struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ThemeByID("neon")
	}
	return ThemeByID(cfg.Theme)
}

// Config is the persisted user settings file.
type Config struct {
	Theme   string `json:"theme"`
	Shuffle bool   `json:"shuffle"`
	Repeat  bool   `json:"repeat"`
	Viz     string `json:"viz"`   // auto | bars | vu | wave | off
	Cover   string `json:"cover"` // auto | sextant | half
}

// LoadConfig reads settings, tolerant to missing/broken files.
func LoadConfig() Config {
	c := Config{Theme: "neon", Viz: "auto", Cover: "auto"}
	p, err := configPath()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	var raw struct {
		Theme   string `json:"theme"`
		Shuffle bool   `json:"shuffle"`
		Repeat  bool   `json:"repeat"`
		Viz     string `json:"viz"`
		Cover   string `json:"cover"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return c
	}
	if raw.Theme != "" {
		c.Theme = raw.Theme
	}
	c.Shuffle, c.Repeat = raw.Shuffle, raw.Repeat
	if raw.Viz != "" {
		c.Viz = raw.Viz
	}
	if raw.Cover != "" {
		c.Cover = raw.Cover
	}
	return c
}

// Save persists the full settings file.
func (c Config) Save() error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// Save persists the theme id.
func (t Theme) Save() error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Theme string `json:"theme"`
	}{Theme: t.ID})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
