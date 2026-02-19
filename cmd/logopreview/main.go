package main

import (
	"fmt"
)

// ANSI color helpers
const (
	orange = "\033[38;2;242;140;40m"
	gray   = "\033[38;5;242m"
	white  = "\033[1;37m"
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
)

func main() {
	info1 := white + "Hawkeye CLI " + gray + "v0.1.0" + reset
	info2 := gray + "localhost:3001 · 6652...c5" + reset

	fmt.Println()
	fmt.Println(bold + "═══ Pick a bird logo ═══" + reset)

	// ── Option A: Hawk profile ──
	fmt.Println()
	fmt.Println(dim + "Option A — Hawk profile" + reset)
	fmt.Println()
	fmt.Printf("     %s▄▄%s\n", gray, reset)
	fmt.Printf("   %s▄█%s%s◉%s %s█%s%s▀▀▀▀▀▀▀▀▌%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info1)
	fmt.Printf("   %s▀██▄█%s%s▄▄▄▄▄▄▄▄%s    %s\n", gray, reset, orange, reset, info2)
	fmt.Printf("     %s▀▀%s\n", gray, reset)

	// ── Option B: Round toucan ──
	fmt.Println()
	fmt.Println(dim + "Option B — Round toucan" + reset)
	fmt.Println()
	fmt.Printf("    %s▄▀▀▀▄%s\n", gray, reset)
	fmt.Printf("   %s▐%s %s◉%s %s▌%s%s▀▀▀▀▀▀▀▌%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info1)
	fmt.Printf("    %s▀▄▄▄▀%s%s▄▄▄▄▄▄▄%s    %s\n", gray, reset, orange, reset, info2)

	// ── Option C: Minimal sleek ──
	fmt.Println()
	fmt.Println(dim + "Option C — Minimal sleek" + reset)
	fmt.Println()
	fmt.Printf("   %s▄██▄%s\n", gray, reset)
	fmt.Printf("   %s█%s%s◉%s%s▐█%s%s▀▀▀▀▀▀▀▀▌%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info1)
	fmt.Printf("   %s▀██▀%s%s▄▄▄▄▄▄▄▄%s    %s\n", gray, reset, orange, reset, info2)

	// ── Option D: Bird face ──
	fmt.Println()
	fmt.Println(dim + "Option D — Bird face" + reset)
	fmt.Println()
	fmt.Printf("   %s▄▀▀▀▄%s  %s▄▄▄▄▄▄▄▀%s\n", gray, reset, orange, reset)
	fmt.Printf("   %s█%s %s◉%s %s█%s  %s████████▌%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info1)
	fmt.Printf("   %s▀▄▄▄▀%s  %s▀▀▀▀▀▀▀▄%s   %s\n", gray, reset, orange, reset, info2)

	// ── Option E: Cute compact ──
	fmt.Println()
	fmt.Println(dim + "Option E — Cute compact" + reset)
	fmt.Println()
	fmt.Printf("   %s▄█▀▀▄%s  %s▄▄▄▄▄▀%s    %s\n", gray, reset, orange, reset, info1)
	fmt.Printf("   %s█%s%s◉%s%s▄▄█%s  %s██████▌%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info2)
	fmt.Printf("    %s▀▀▀%s   %s▀▀▀▀▀▄%s\n", gray, reset, orange, reset)

	// ── Option F: Side profile bird ──
	fmt.Println()
	fmt.Println(dim + "Option F — Side profile" + reset)
	fmt.Println()
	fmt.Printf("    %s▄▄▄%s\n", gray, reset)
	fmt.Printf("   %s█%s%s◉%s%s▀▀█%s%s━━━━━━━▶%s   %s\n", gray, reset, white, reset, gray, reset, orange, reset, info1)
	fmt.Printf("   %s▀█▄▄▀%s              %s\n", gray, reset, info2)

	fmt.Println()
	fmt.Println(dim + "Which one? (A/B/C/D/E/F)" + reset)
	fmt.Println()
}
