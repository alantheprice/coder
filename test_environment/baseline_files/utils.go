package main

import (
	"strings"
	"strconv"
)

// Utility functions that need unit tests

// FormatName capitalizes first letter of each word
func FormatName(name string) string {
	if name == "" {
		return ""
	}
	
	words := strings.Split(name, " ")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// CalculateTotal adds up a list of price strings and returns formatted total
func CalculateTotal(prices []string) (string, error) {
	total := 0.0
	for _, price := range prices {
		// Remove $ sign if present
		cleanPrice := strings.TrimPrefix(price, "$")
		value, err := strconv.ParseFloat(cleanPrice, 64)
		if err != nil {
			return "", err
		}
		total += value
	}
	return fmt.Sprintf("$%.2f", total), nil
}

// ValidateEmail checks if an email has basic valid format
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	
	return strings.Contains(parts[1], ".")
}