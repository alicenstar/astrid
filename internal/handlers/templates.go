package handlers

import (
	"html/template"
	"path/filepath"

	"github.com/alicenstar/astrid/internal/models"
)

func LoadTemplates(templatesDir string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"dayTarget": func(days []models.CaloriePlanDay, dayOfWeek int) *int {
			for _, d := range days {
				if d.DayOfWeek == dayOfWeek {
					return &d.CalorieTarget
				}
			}
			return nil
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"pct": func(current, target int) int {
			if target == 0 {
				return 0
			}
			return current * 100 / target
		},
		"progressColor": func(current, target int) string {
			if target == 0 {
				return "green"
			}
			pct := current * 100 / target
			if pct <= 90 {
				return "green"
			}
			if pct <= 105 {
				return "yellow"
			}
			return "red"
		},
	}

	pattern := filepath.Join(templatesDir, "*.html")
	partialsPattern := filepath.Join(templatesDir, "partials", "*.html")

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	tmpl, err = tmpl.ParseGlob(partialsPattern)
	if err != nil {
		// No partials yet is ok
		return tmpl, nil
	}
	return tmpl, nil
}
