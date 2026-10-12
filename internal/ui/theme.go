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
	AppBG         string
	Grade         cover.Grade
	Viz           VizStyle
	RoundedBorder bool
}

// AllThemes returns all 6 themes in fixed order.
func AllThemes() []Theme {
	return []Theme{
		{
			ID: "neon", Title: "#00FFFF", Text: "#E8E8F0", Dim: "#888888",
			Accent: "#FF5FD2", Accent2: "#00FF9D", Peak: "#FF5FD2",
			SelBG: "#1a1a2e", BorderFG: "#7D5CFF", AppBG: "",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: true,
		},
		{
			ID: "lamp", Title: "#FFB000", Text: "#F5E6C8", Dim: "#8A6D4B",
			Accent: "#FFB000", Accent2: "#FF6A00", Peak: "#FF3B30",
			SelBG: "#2A1E0E", BorderFG: "#FFB000", AppBG: "#1A120B",
			Grade: cover.GradeSepia, Viz: VizVU, RoundedBorder: false,
		},
		{
			ID: "minimal", Title: "#EEEEEE", Text: "#CCCCCC", Dim: "#777777",
			Accent: "#EEEEEE", Accent2: "#999999", Peak: "#FFFFFF",
			SelBG: "#222222", BorderFG: "#444444", AppBG: "#111111",
			Grade: cover.GradeMono, Viz: VizWave, RoundedBorder: false,
		},
		{
			ID: "phosphor", Title: "#66FF66", Text: "#33FF33", Dim: "#1A7A1A",
			Accent: "#66FF66", Accent2: "#33FF33", Peak: "#FFFF66",
			SelBG: "#0E1F0E", BorderFG: "#1F5F1F", AppBG: "#0A0F0A",
			Grade: cover.GradePhosphor, Viz: VizBars, RoundedBorder: true,
		},
		{
			ID: "paper", Title: "#1A1A1A", Text: "#1A1A1A", Dim: "#8A8578",
			Accent: "#B3541E", Accent2: "#7A3B12", Peak: "#C22F2F",
			SelBG: "#E4DCC8", BorderFG: "#C9C2B2", AppBG: "#F5F0E6",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: false,
		},
		{
			ID: "midnight", Title: "#7DD3FC", Text: "#DCE6F5", Dim: "#5B6B8C",
			Accent: "#7DD3FC", Accent2: "#3B82F6", Peak: "#F472B6",
			SelBG: "#16213B", BorderFG: "#274068", AppBG: "#0B1026",
			Grade: cover.GradeNone, Viz: VizBars, RoundedBorder: true,
		},
	}
}

// ThemeByID returns the theme with the given id, or neon for unknown ids.
func ThemeByID(id string) Theme {
	for _, t := range AllThemes() {
		if t.ID == id {
			return t
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

// LoadTheme reads ~/.config/mudex/config.json {"theme":"<id>"}, neon on any error.
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
	if cfg.Theme == "" {
		return ThemeByID("neon")
	}
	return ThemeByID(cfg.Theme)
}

// Save writes the current theme ID to ~/.config/mudex/config.json.
func (t Theme) Save() error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"theme": t.ID})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
