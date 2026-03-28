package main

import (
	"fmt"
	"log"

	"github.com/charmbracelet/huh"
)

// RunSetupWizard walks the user through first-time configuration.
func RunSetupWizard() (*AppConfig, error) {
	cfg := &AppConfig{}

	fmt.Println()
	fmt.Println("  ██████╗ ███████╗ ██████╗ ██████╗ ███╗   ██╗")
	fmt.Println("  ██╔══██╗██╔════╝██╔════╝██╔═══██╗████╗  ██║")
	fmt.Println("  ██████╔╝█████╗  ██║     ██║   ██║██╔██╗ ██║")
	fmt.Println("  ██╔══██╗██╔══╝  ██║     ██║   ██║██║╚██╗██║")
	fmt.Println("  ██║  ██║███████╗╚██████╗╚██████╔╝██║ ╚████║")
	fmt.Println("  ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝")
	fmt.Println()
	fmt.Println("  Welcome to Recon — your daily cyber & tech intelligence feed.")
	fmt.Println("  Let's configure your preferences. This only runs once.")
	fmt.Println()

	// Step 1: Timezone selection
	var tzOptions []huh.Option[string]
	for _, tz := range Timezones {
		tzOptions = append(tzOptions, huh.NewOption(tz.Label, tz.Value))
	}

	var selectedTZ string
	tzForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select your timezone").
				Options(tzOptions...).
				Value(&selectedTZ),
		),
	)

	if err := tzForm.Run(); err != nil {
		return nil, fmt.Errorf("timezone selection failed: %w", err)
	}
	cfg.Timezone = selectedTZ

	// Step 2: Category multi-select
	var catOptions []huh.Option[string]
	for _, cat := range AllCategories {
		catOptions = append(catOptions, huh.NewOption(cat.Name, cat.ID))
	}

	var selectedCategories []string
	catForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Pick your default focus areas").
				Description("Space to toggle, Enter to confirm").
				Options(catOptions...).
				Value(&selectedCategories),
		),
	)

	if err := catForm.Run(); err != nil {
		return nil, fmt.Errorf("category selection failed: %w", err)
	}

	if len(selectedCategories) == 0 {
		log.Println("No categories selected. Defaulting to Vulnerabilities & CVEs.")
		selectedCategories = []string{"vulnerabilities"}
	}

	cfg.Categories = selectedCategories
	cfg.Keywords = KeywordsForCategories(selectedCategories)

	// Step 3: Schedule time
	var scheduleTime string
	var skipSchedule bool

	scheduleForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("When should your daily digest auto-run?").
				Description("Enter time in 24h format (e.g. 07:00). Leave empty to skip.").
				Placeholder("07:00").
				Value(&scheduleTime),
			huh.NewConfirm().
				Title("Skip auto-scheduling?").
				Value(&skipSchedule),
		),
	)

	if err := scheduleForm.Run(); err != nil {
		return nil, fmt.Errorf("schedule setup failed: %w", err)
	}

	if !skipSchedule && scheduleTime != "" {
		cfg.ScheduleTime = scheduleTime
		fmt.Printf("\n  ✓ Will auto-run daily at %s (%s)\n", scheduleTime, selectedTZ)
	} else {
		cfg.ScheduleTime = ""
		fmt.Println("\n  ✓ Auto-scheduling skipped. Run `recon` manually whenever you want.")
	}

	cfg.SetupComplete = true

	// Save config
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Install systemd timer if scheduled
	if cfg.ScheduleTime != "" {
		fmt.Println("  → Installing systemd timer...")
		if err := ScheduleInstall(cfg.ScheduleTime); err != nil {
			fmt.Printf("  ⚠ Could not install timer: %v\n", err)
			fmt.Println("    You can retry later with `recon schedule --time " + cfg.ScheduleTime + "`")
		} else {
			fmt.Println("  ✓ Systemd timer installed successfully.")
		}
	}

	fmt.Println()
	fmt.Println("  Setup complete! Run `recon` to fetch your first digest.")
	fmt.Println()

	return cfg, nil
}
