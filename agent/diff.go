package agent

import (
	"fmt"
	"strings"
)

// ShowColoredDiff displays a colored diff between old and new content, focusing on actual changes
func (a *Agent) ShowColoredDiff(oldContent, newContent string, maxLines int) {
	const red = "\033[31m"    // Red for deletions
	const green = "\033[32m"  // Green for additions
	const reset = "\033[0m"
	
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	// Find the actual changes by identifying differing regions
	changes := a.findChanges(oldLines, newLines)
	
	if len(changes) == 0 {
		fmt.Println("No changes detected")
		return
	}
	
	fmt.Println("File changes:")
	fmt.Println("----------------------------------------")
	
	totalLinesShown := 0
	
	for _, change := range changes {
		if totalLinesShown >= maxLines {
			fmt.Printf("... (truncated after %d lines)\n", maxLines)
			break
		}
		
		// Show deletions (old content)
		if change.OldLength > 0 {
			for i := 0; i < change.OldLength && totalLinesShown < maxLines; i++ {
				lineNum := change.OldStart + i
				if lineNum < len(oldLines) {
					fmt.Printf("%s- %s%s\n", red, oldLines[lineNum], reset)
					totalLinesShown++
				}
			}
		}
		
		// Show additions (new content)
		if change.NewLength > 0 {
			for i := 0; i < change.NewLength && totalLinesShown < maxLines; i++ {
				lineNum := change.NewStart + i
				if lineNum < len(newLines) {
					fmt.Printf("%s+ %s%s\n", green, newLines[lineNum], reset)
					totalLinesShown++
				}
			}
		}
		
		// Add separator between changes
		if totalLinesShown < maxLines {
			fmt.Println()
			totalLinesShown++
		}
	}
	
	fmt.Println("----------------------------------------")
}


// findChanges identifies regions where content differs between old and new versions
func (a *Agent) findChanges(oldLines, newLines []string) []DiffChange {
	var changes []DiffChange
	
	oldLen := len(oldLines)
	newLen := len(newLines)
	maxLen := oldLen
	if newLen > oldLen {
		maxLen = newLen
	}
	
	changeStart := -1
	
	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""
		
		if i < oldLen {
			oldLine = oldLines[i]
		}
		if i < newLen {
			newLine = newLines[i]
		}
		
		// Check if lines differ
		linesDiffer := oldLine != newLine
		
		if linesDiffer {
			// Start of a new change
			if changeStart == -1 {
				changeStart = i
			}
		} else {
			// End of a change (if we were in one)
			if changeStart != -1 {
				// Calculate the lengths for old and new content
				oldChangeLen := i - changeStart
				newChangeLen := i - changeStart
				
				// Adjust lengths if one side runs out of lines
				if changeStart + oldChangeLen > oldLen {
					oldChangeLen = oldLen - changeStart
				}
				if changeStart + newChangeLen > newLen {
					newChangeLen = newLen - changeStart
				}
				
				// Ensure lengths are not negative
				if oldChangeLen < 0 {
					oldChangeLen = 0
				}
				if newChangeLen < 0 {
					newChangeLen = 0
				}
				
				changes = append(changes, DiffChange{
					OldStart:  changeStart,
					OldLength: oldChangeLen,
					NewStart:  changeStart,
					NewLength: newChangeLen,
				})
				
				changeStart = -1 // Reset for next change
			}
		}
	}
	
	// Handle case where change extends to the end
	if changeStart != -1 {
		oldChangeLen := oldLen - changeStart
		newChangeLen := newLen - changeStart
		
		if oldChangeLen < 0 {
			oldChangeLen = 0
		}
		if newChangeLen < 0 {
			newChangeLen = 0
		}
		
		changes = append(changes, DiffChange{
			OldStart:  changeStart,
			OldLength: oldChangeLen,
			NewStart:  changeStart,
			NewLength: newChangeLen,
		})
	}
	
	return changes
}