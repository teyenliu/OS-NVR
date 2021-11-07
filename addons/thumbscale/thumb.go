package thumbscale

import (
	"fmt"
	"nvr"
	"nvr/pkg/monitor"
	"os"
	"strings"
)

func init() {
	nvr.RegisterTplHook(modifyTemplates)
	nvr.RegisterMonitorRecSaveHook(onRecSave)
}

func modifyTemplates(pageFiles map[string]string) error {
	js, exists := pageFiles["settings.js"]
	if !exists {
		return fmt.Errorf("motion: settings.js: %w", os.ErrNotExist)
	}

	pageFiles["settings.js"] = modifySettingsjs(js)
	return nil
}

func modifySettingsjs(tpl string) string {
	const target = "timestampOffset: fieldTemplate.integer("

	const javascript = `
 		thumbScale:fieldTemplate.select(
			"Thumbnail scale",
			["full", "half", "third", "quarter", "sixth", "eighth"],
			"full",
		),`

	return strings.ReplaceAll(tpl, target, javascript+target)
}

func onRecSave(m *monitor.Monitor, args *string) {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	scale := parseScale(m.Config["thumbScale"])
	if scale == "1" {
		return
	}

	// Inject filter into args.
	target := " -frames"
	filter := genFilter(scale)
	*args = strings.ReplaceAll(*args, target, filter+target)
}

func parseScale(scale string) string {
	switch strings.ToLower(scale) {
	case "full":
		return "1"
	case "half":
		return "2"
	case "third":
		return "3"
	case "quarter":
		return "4"
	case "sixth":
		return "6"
	case "eighth":
		return "8"
	default:
		return "1"
	}
}

// Generate filter argument that divide height and width by ratio.
func genFilter(ratio string) string {
	// OUTPUT: -vf scale='iw/r:ih/2'
	r := ratio
	return " -vf scale='iw/" + r + ":ih/" + r + "'"
}
