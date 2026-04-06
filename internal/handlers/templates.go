package handlers

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/alicenstar/astrid/internal/models"
)

type Templates struct {
	pages map[string]*template.Template
}

func (t *Templates) Render(name string, data any) (*template.Template, error) {
	tmpl, ok := t.pages[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return tmpl, nil
}

func LoadTemplates(templatesDir string) (*Templates, error) {
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

	layoutFile := filepath.Join(templatesDir, "layout.html")
	pages := []string{"dashboard", "plans", "log", "summary", "workouts"}

	t := &Templates{pages: make(map[string]*template.Template)}

	for _, page := range pages {
		pageFile := filepath.Join(templatesDir, page+".html")
		tmpl, err := template.New("").Funcs(funcMap).ParseFiles(layoutFile, pageFile)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		t.pages[page] = tmpl
	}

	return t, nil
}
