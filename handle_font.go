package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

//listFonts 获取字体服务列表
func listFonts(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(identityKey)
	if uid != ATLAS {
		uid = ATLAS
	}
	set := userSet.service(uid)
	if set == nil {
		log.Warnf(`listFonts, %s's service not found ^^`, uid)
		res.Fail(c, 4043)
		return
	}
	var fonts []*Font
	set.F.Range(func(_, v interface{}) bool {
		fonts = append(fonts, v.(*Font))
		return true
	})
	res.DoneData(c, fonts)
}

//uploadStyle 上传新样式
func uploadFont(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(identityKey)
	if uid != ATLAS {
		res.FailMsg(c, "no perm")
		return
	}
	set := userSet.service(uid)
	if set == nil {
		log.Warnf(`uploadFont, %s's service not found ^^`, uid)
		res.Fail(c, 4043)
		return
	}
	// style source
	file, err := c.FormFile("file")
	if err != nil {
		log.Warnf(`uploadFont, %s get file error, details: %s`, uid, err)
		res.Fail(c, 4048)
		return
	}
	ext := filepath.Ext(file.Filename)
	dst := filepath.Join(viper.GetString("paths.fonts"), uid, file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		log.Errorf(`uploadFont, save %s's file error, details: %s`, uid, err)
		res.Fail(c, 5002)
		return
	}
	lext := strings.ToLower(ext)
	switch lext {
	case ZIPEXT, ".pbfonts":
	default:
		log.Errorf(`uploadFont, %s's font format error (%s)`, uid, file.Filename)
		res.FailMsg(c, "上传格式错误,请上传zip/pbfonts格式")
		return
	}
	if lext == ZIPEXT {
		outdir := strings.TrimSuffix(dst, filepath.Ext(dst))
		err := UnZipToDir(dst, outdir)
		if err != nil {
			res.FailMsg(c, "上传文件解压缩错误")
			return
		}
	}
	font, err := LoadFont(dst)
	if err != nil {
		log.Errorf("uploadFont, could not load font %s, details: %s", dst, err)
		res.FailErr(c, err)
		return
	}
	//入库
	err = font.UpInsert()
	if err != nil {
		log.Errorf(`uploadFont, upinsert font %s error, details: %s`, font.ID, err)
		res.FailErr(c, err)
		return
	}

	err = font.Service()
	if err != nil {
		log.Errorf(`uploadFont,serve font (%s) to service error, details: %s`, font.ID, err)
		res.FailErr(c, err)
		return
	}
	set.F.Store(font.ID, font)
	res.DoneData(c, font)
}

//upInsertStyle create a style
func deleteFonts(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(identityKey)
	if uid != ATLAS {
		res.FailMsg(c, "no permission")
		return
	}
	set := userSet.service(uid)
	if set == nil {
		log.Warnf(`deleteFonts, %s's service not found ^^`, uid)
		res.Fail(c, 4043)
		return
	}
	fontstack := c.Param("fontstack")
	fonts := strings.Split(fontstack, ",")
	for _, fid := range fonts {
		font := userSet.font(uid, fid)
		if font == nil {
			log.Errorf(`deleteFonts, %s's font service (%s) not found ^^`, uid, fid)
			res.Fail(c, 4047)
			return
		}
		set.F.Delete(fid)
		err := db.Delete(&Font{}).Where("id = ?", fid).Error
		if err != nil {
			log.Error(err)
			res.FailErr(c, err)
			return
		}
		err = os.Remove(font.URL)
		if err != nil {
			log.Errorf(`deleteStyle, remove %s's style .zip (%s) error, details:%s ^^`, uid, fid, err)
		}
		dir := strings.TrimSuffix(font.URL, ".pbfonts")
		err = os.RemoveAll(dir)
		if err != nil {
			log.Errorf(`deleteStyle, remove %s's style dir (%s) error, details:%s ^^`, uid, fid, err)
		}
	}
	res.Done(c, "")
}

//getGlyphs 获取字体pbf,需区别于数据pbf,开启gzip压缩以加快传输,get glyph pbf.
func getGlyphs(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(ATLAS)
	if set == nil {
		res.Fail(c, 4046)
		return
	}
	fontstack := c.Param("fontstack")
	fontrange := c.Param("range")
	rgPat := `[\d]+-[\d]+.pbf$`
	if ok, _ := regexp.MatchString(rgPat, fontrange); !ok {
		log.Warnf("getGlyphs, range pattern error; range:%s; user:%s", fontrange, uid)
		res.Fail(c, 4005)
		return
	}

	var pbf []byte
	fonts := strings.Split(fontstack, ",")
	switch len(fonts) {
	case 0:
		log.Errorf("getGlyphs, fontstack is nil ~")
		res.Fail(c, 4005)
		return
	case 1:
		iv, ok := set.F.Load(fontstack)
		if !ok {
			log.Errorf("getGlyphs, fontstack is not found ~")
			res.Fail(c, 4047)
			return
		}
		data, err := iv.(*Font).Font(fontrange)
		if err != nil {
			log.Errorf("getGlyphs, get pbf font error, details:%s ~", err)
			res.Fail(c, 4005)
			return
		}
		pbf = data

	default: //multi fonts

		var fs []*Font
		hasdefault := false
		haslost := false
		for _, font := range fonts {
			if font == DEFAULTFONT {
				hasdefault = true
			}

			iv, ok := set.F.Load(font)
			if !ok {
				haslost = true
				continue
			}
			fs = append(fs, iv.(*Font))
		}
		//没有默认字体且有丢失字体,则加载默认字体
		if !hasdefault && haslost {
			iv, ok := set.F.Load(DEFAULTFONT)
			if ok {
				f, ok := iv.(*Font)
				if ok {
					fs = append(fs, f)
				}
			}
		}

		contents := make([][]byte, len(fs))
		var wg sync.WaitGroup
		//need define func, can't use sugar ":="
		getFontPBF := func(f *Font, fontrange string, index int) {
			//fallbacks unchanging
			defer wg.Done()
			err := f.db.QueryRow("select data from fonts where range = ?", fontrange).Scan(&contents[index])
			if err != nil {
				log.Error(err)
				if err == sql.ErrNoRows {
					return
				}
				return
			}
		}
		for i, f := range fs {
			wg.Add(1)
			go getFontPBF(f, fontrange, i)
		}
		wg.Wait()

		//if  getFontPBF can't get content,the buffer array is nil, remove the nils
		var buffers [][]byte
		var bufFonts []string
		for i, buf := range contents {
			if nil == buf {
				continue
			}
			buffers = append(buffers, buf)
			bufFonts = append(bufFonts, fonts[i])
		}
		if len(buffers) != len(bufFonts) {
			log.Error("len(buffers) != len(fonts)")
		}
		if 0 == len(buffers) {
			log.Errorf("getGlyphs, empty pbf font ~")
			res.Fail(c, 4005)
			return
		}
		if 1 == len(buffers) {
			pbf = buffers[0]
		} else {
			c, err := Combine(buffers, bufFonts)
			if err != nil {
				log.Error("combine buffers error:", err)
				pbf = buffers[0]
			} else {
				pbf = c
			}
		}
	}

	lastModified := time.Now().UTC().Format("2006-01-02 03:04:05 PM")
	c.Header("Content-Type", "application/x-protobuf")
	c.Header("Last-Modified", lastModified)
	c.Writer.Write(pbf)
	return
	// gz, err := gzip.NewWriterLevel(c.Writer, gzip.DefaultCompression)
	// if err != nil {
	// c.Writer.Write(pbf)
	// return
	// }
	// defer func() {
	// c.Header("Content-Length", "0")
	// gz.Close()
	// }()
	// gz.Write(pbf)
	// c.Header("Content-Encoding", "gzip")
	// c.Header("Vary", "Accept-Encoding")
	// return
}
