// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package api

import (
	"crypto/sha1"
	"encoding/binary"
	"github.com/alvinbaena/pwd-checker/gcs"
	"github.com/gin-gonic/gin"
	"github.com/nbutton23/zxcvbn-go"
	"net/http"
	"regexp"
	"strings"
)

type queryApi struct {
	searcher *gcs.Reader
}

func (q *queryApi) checkPassword(c *gin.Context) {
	var req queryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h := sha1.New()
	h.Write([]byte(req.Password))
	buf := h.Sum(nil)
	hash := binary.BigEndian.Uint64(buf)
	exists, err := q.searcher.Exists(hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	entropy := zxcvbn.PasswordStrength(req.Password, nil)
	resp := queryResponse{
		Pwned: exists,
		Strength: &passwordStrength{
			CrackTime:        entropy.CrackTime,
			CrackTimeDisplay: entropy.CrackTimeDisplay,
			Score:            entropy.Score,
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (q *queryApi) checkHash(c *gin.Context) {
	var req hashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if match, _ := regexp.MatchString("^[a-fA-F\\d]{40}$", strings.ToUpper(req.Hash)); !match {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input is not a valid SHA1 Hexadecimal hash"})
		return
	}

	hash := gcs.U64FromHex([]byte(strings.ToUpper(req.Hash))[0:16])
	exists, err := q.searcher.Exists(hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, queryResponse{Pwned: exists})
}

func RegisterQueryApi(group *gin.RouterGroup, fileName string) error {
	searcher := gcs.NewReader(fileName)
	if err := searcher.Initialize(); err != nil {
		return err
	}

	q := &queryApi{searcher: searcher}

	group.POST("/password", q.checkPassword)
	group.POST("/hash", q.checkHash)

	return nil
}
