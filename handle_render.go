package main

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

func renderSignup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", gin.H{
		"Title": "AtlasMap",
	})
}

func renderSignin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "AtlasMap",
	})
}

func renderForgot(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot.html", gin.H{
		"Title": "AtlasMap",
	})
}

func renderReset(c *gin.Context) {
	c.HTML(http.StatusOK, "reset.html", gin.H{
		"Title": "AtlasMap",
		"User":  c.Param("user"),
		"Token": c.Param("token"),
	}) // can't handle /login/reset/:email:token
}

func renderAccount(c *gin.Context) {
	res := NewRes()
	id := c.GetString(identityKey)
	user := &User{}
	if err := db.Where("name = ?", id).First(&user).Error; err != nil {
		res.FailMsg(c, fmt.Sprintf("renderAccount, get user info: %s; user: %s", err, id))
		if !gorm.IsRecordNotFoundError(err) {
			log.Errorf("renderAccount, get user info: %s; user: %s", err, id)
		}
		return
	}

	c.HTML(http.StatusOK, "account.html", user)
}

func renderUpdateUser(c *gin.Context) {
	res := NewRes()
	id := c.GetString(identityKey)
	user := &User{}
	if err := db.Where("name = ?", id).First(&user).Error; err != nil {
		res.FailMsg(c, fmt.Sprintf("renderAccount, get user info: %s; user: %s", err, id))
		if !gorm.IsRecordNotFoundError(err) {
			log.Errorf("renderAccount, get user info: %s; user: %s", err, id)
		}
		return
	}

	c.HTML(http.StatusOK, "update.html", user)
}

func renderChangePassword(c *gin.Context) {
	c.HTML(http.StatusOK, "change.html", gin.H{
		"Title": "AtlasMap",
	}) // can't handle /login/reset/:email:token
}

func renderMapsImport(c *gin.Context) {
	id := c.GetString(identityKey)
	c.HTML(http.StatusOK, "import-m.html", gin.H{
		"Title": "AtlasMap",
		"User":  id,
	})
}

func studioIndex(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(identityKey)
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	var styles []*Style
	set.S.Range(func(_, v interface{}) bool {
		styles = append(styles, v.(*Style))
		return true
	})
	var tss []*Tileset
	set.T.Range(func(_, v interface{}) bool {
		tss = append(tss, v.(*Tileset))
		return true
	})

	//public
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Title":    "AtlasMap",
		"Login":    false,
		"Styles":   styles,
		"Tilesets": tss,
	})
}

func studioEditer(c *gin.Context) {
	//public
	res := NewRes()
	uid := c.GetString(identityKey)
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	var styles []*Style
	set.S.Range(func(_, v interface{}) bool {
		styles = append(styles, v.(*Style))
		return true
	})

	var tss []*Tileset
	set.T.Range(func(_, v interface{}) bool {
		tss = append(tss, v.(*Tileset))
		return true
	})
	c.HTML(http.StatusOK, "editor.html", gin.H{
		"Title":    "Creater",
		"User":     uid,
		"Styles":   styles,
		"Tilesets": tss,
	})
}

func renderStyleUpload(c *gin.Context) {
	c.HTML(http.StatusOK, "upload-s.html", gin.H{
		"Title": "AtlasMap",
	})
}

func renderSpriteUpload(c *gin.Context) {
	c.HTML(http.StatusOK, "upload-ss.html", gin.H{
		"Title": "AtlasMap",
		"id":    c.Param("id"),
	})
}

func renderTilesetsUpload(c *gin.Context) {
	id := c.GetString(identityKey)
	c.HTML(http.StatusOK, "upload-t.html", gin.H{
		"Title": "AtlasMap",
		"User":  id,
	})
}

func renderDatasetsUpload(c *gin.Context) {
	id := c.GetString(identityKey)
	c.HTML(http.StatusOK, "upload-d.html", gin.H{
		"Title": "AtlasMap",
		"User":  id,
	})
}
