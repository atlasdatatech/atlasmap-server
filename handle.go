package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func index(c *gin.Context) {
	_, err := authMid.GetClaimsFromJWT(c)
	if err != nil {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title": "AtlasMap",
			"Login": true,
		})
	}
	c.Redirect(http.StatusFound, "/studio/")
}

func ping(c *gin.Context) {
	res := NewRes()
	err := db.DB().Ping()
	if err != nil {
		res.FailErr(c, err)
		return
	}
	res.DoneData(c, gin.H{
		"status": "db pong ~",
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	})
}

func crsList(c *gin.Context) {
	res := NewRes()
	res.DoneData(c, CRSs)
}

func encodingList(c *gin.Context) {
	res := NewRes()
	res.DoneData(c, Encodings)
}

func fieldTypeList(c *gin.Context) {
	res := NewRes()
	res.DoneData(c, FieldTypes)
}
