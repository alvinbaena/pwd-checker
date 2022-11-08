// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package api

type queryRequest struct {
	Password string `json:"password" binding:"required"`
}

type queryResponse struct {
	Pwned    bool              `json:"pwned"`
	Strength *passwordStrength `json:"strength,omitempty"`
}

type hashRequest struct {
	Hash string `json:"hash" binding:"required"`
}

// MinEntropyMatch is the lowest entropy match found
type passwordStrength struct {
	CrackTime        float64 `json:"crackTime"`
	CrackTimeDisplay string  `json:"crackTimeDisplay"`
	Score            int     `json:"score"`
}
