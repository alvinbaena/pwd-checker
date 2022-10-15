package api

type queryRequest struct {
	Password string `json:"password" binding:"required"`
}

type queryResponse struct {
	Pwned    bool    `json:"pwned"`
	Strength *string `json:"strength"`
}

type hashRequest struct {
	Hash string `json:"hash" binding:"required"`
}
