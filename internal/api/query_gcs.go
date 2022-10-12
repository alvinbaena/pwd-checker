package api

import (
	"crypto/sha1"
	"encoding/binary"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"pwd-checker/internal/gcs"
	"regexp"
	"strings"
)

type queryRequest struct {
	Password string `json:"password" binding:"required"`
}

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

	c.JSON(http.StatusOK, gin.H{"exists": exists})
}

func (q *queryApi) checkHash(c *gin.Context) {
	var req queryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if match, _ := regexp.MatchString("^[a-fA-F\\d]{40}$", strings.ToUpper(req.Password)); !match {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input is not a valid SHA1 Hexadecimal hash"})
		return
	}

	hash := gcs.U64FromHex([]byte(strings.ToUpper(req.Password))[0:16])
	exists, err := q.searcher.Exists(hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exists": exists})
}

func RegisterQueryApi(group *gin.RouterGroup, file *os.File) error {
	searcher := gcs.NewReader(file)
	if err := searcher.Initialize(); err != nil {
		return err
	}

	q := &queryApi{searcher: searcher}

	group.POST("/password", q.checkPassword)
	group.POST("/hash", q.checkHash)

	return nil
}
