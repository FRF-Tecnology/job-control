//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func prioridade() string {
	c, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		return "erro"
	}
	return fmt.Sprintf("0x%x", c)
}
