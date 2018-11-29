package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	log "github.com/sirupsen/logrus"
	gomail "gopkg.in/gomail.v2"
)

var codes = map[int]string{
	0: "检查消息",

	200: "成功",
	201: "已创建",
	202: "已接受",
	204: "无内容",

	300: "重定向",

	400:  "请求无法解析",
	4001: "必填参数校验错误",
	4002: "达到最大尝试登录次数,稍后再试",
	4003: "瓦片请求格式错误",
	4004: "符号请求格式错误",
	4005: "字体请求格式错误",

	401:  "未授权",
	4011: "用户名或密码错误",
	4012: "用户名或密码非法",

	403:  "禁止访问",
	4031: "用户已存在",

	404:  "找不到资源",
	4041: "用户不存在",
	4042: "角色不存在",
	4043: "地图不存在",
	4044: "服务不存在",
	4045: "找不到数据集",
	4046: "找不到上传文件",

	408: "请求超时",

	500:  "系统错误",
	5001: "数据库错误",
	5002: "文件读写错误",
	5003: "IO读写错误",
	5004: "MBTiles读写错误",
	5005: "系统配置错误",

	501: "维护中",
	503: "服务不可用",
}

//Res response schema
type Res struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

//NewRes Create Res
func NewRes() *Res {
	return &Res{
		Code: http.StatusOK,
		Msg:  codes[http.StatusOK],
	}
}

//Fail failed error
func (res *Res) Fail(c *gin.Context, code int) {
	res.Code = code
	res.Msg = codes[code]
	c.JSON(http.StatusOK, res)
}

//FailErr failed string
func (res *Res) FailErr(c *gin.Context, err error) {
	res.Code = 0
	if err != nil {
		res.Msg = err.Error()
	}
	c.JSON(http.StatusOK, res)
}

//FailMsg failed string
func (res *Res) FailMsg(c *gin.Context, msg string) {
	res.Code = 0
	res.Msg = msg
	c.JSON(http.StatusOK, res)
}

//Done done
func (res *Res) Done(c *gin.Context, msg string) {
	res.Code = http.StatusOK
	res.Msg = codes[http.StatusOK]
	if msg != "" {
		res.Msg = msg
	}
	c.JSON(http.StatusOK, res)
}

//DoneCode done
func (res *Res) DoneCode(c *gin.Context, code int) {
	res.Code = code
	res.Msg = codes[code]
	c.JSON(http.StatusOK, res)
}

//DoneData done
func (res *Res) DoneData(c *gin.Context, data interface{}) {
	res.Code = http.StatusOK
	res.Msg = codes[http.StatusOK]
	res.Data = data
	c.JSON(http.StatusOK, res)
}

//Reset reset to init
func (res *Res) Reset() {
	res.Code = http.StatusOK
	res.Msg = codes[http.StatusOK]
}

//MailConfig email config and data
type MailConfig struct {
	From     string
	ReplyTo  string
	Subject  string
	TextPath string
	HTMLPath string
	Data     interface{}
}

//SendMail send email
func (conf *MailConfig) SendMail() (err error) {
	m := gomail.NewMessage()

	m.SetHeader("From", conf.From)
	m.SetHeader("To", conf.ReplyTo)
	m.SetHeader("Subject", conf.Subject)
	m.SetHeader("ReplyTo", conf.ReplyTo)

	m.AddAlternativeWriter("text/html", func(w io.Writer) error {
		return template.Must(template.ParseFiles(conf.HTMLPath)).Execute(w, conf.Data)
	})

	d := gomail.NewDialer(cfgV.GetString("smtp.credentials.host"), 587, cfgV.GetString("smtp.credentials.user"), cfgV.GetString("smtp.credentials.password"))
	return d.DialAndSend(m)
}

func getEscapedString(str string) string {
	return strings.Replace(url.QueryEscape(str), "+", "%20", -1)
}

var rSlugify1, _ = regexp.Compile(`[^\w ]+`)
var rSlugify2, _ = regexp.Compile(` +`)

var rUsername, _ = regexp.Compile(`^[a-zA-Z0-9\-\_]+$`)
var rEmail, _ = regexp.Compile(`^[a-zA-Z0-9\-\_\.\+]+@[a-zA-Z0-9\-\_\.]+\.[a-zA-Z0-9\-\_]+$`)

var signupProviderReg, _ = regexp.Compile(`/[^a-zA-Z0-9\-\_]/g`)

/**
preparing id
*/

func slugify(str string) string {
	str = strings.ToLower(str)
	str = rSlugify1.ReplaceAllString(str, "")
	str = rSlugify2.ReplaceAllString(str, "-")
	return str
}

func slugifyName(str string) string {
	str = strings.TrimSpace(str)
	return rSlugify2.ReplaceAllString(str, " ")
}

//XHR xmlhttprequest
func XHR(c *gin.Context) bool {
	return strings.ToLower(c.Request.Header.Get("X-Requested-With")) == "xmlhttprequest"
}

func generateToken(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return b
	}
	token := make([]byte, n*2)
	hex.Encode(token, b)
	return token
}

func createPaths(name string) {
	styles := cfgV.GetString("assets.styles")
	fonts := cfgV.GetString("assets.fonts")
	tilesets := cfgV.GetString("assets.tilesets")
	datasets := cfgV.GetString("assets.datasets")
	os.MkdirAll(styles, os.ModePerm)
	os.MkdirAll(tilesets, os.ModePerm)
	os.MkdirAll(datasets, os.ModePerm)
	os.MkdirAll(fonts, os.ModePerm)
}

func checkUser(uid string) int {
	if uid == "" {
		return 4001
	}
	user := &User{}
	if err := db.Where("name = ?", uid).First(&user).Error; err != nil {
		if !gorm.IsRecordNotFoundError(err) {
			log.Error(err)
			return 5001
		}
		return 4041
	}
	return 200
}

func checkRole(rid string) int {
	if rid == "" {
		return 4001
	}
	role := &Role{}
	if err := db.Where("id = ?", rid).First(&role).Error; err != nil {
		if !gorm.IsRecordNotFoundError(err) {
			log.Error(err)
			return 5001
		}
		return 4042
	}
	return 200
}

func checkMap(mid string) int {
	if mid == "" {
		return 4001
	}
	m := &Map{}
	if err := db.Where("id = ?", mid).First(&m).Error; err != nil {
		if !gorm.IsRecordNotFoundError(err) {
			log.Error(err)
			return 5001
		}
		return 4043
	}
	return 200
}

func checkDataset(did string) int {
	if did == "" {
		return 4001
	}
	if !db.HasTable(did) {
		return 4045
	}
	return 200
}

func buffering(name string, r float64) int {

	db.Exec(`DROP TABLE if EXISTS buffers;`)
	if "banks" != name {
		err := db.Exec(fmt.Sprintf(`CREATE TABLE buffers AS 
		SELECT 机构号,名称,st_buffer(geom::geography,%f)::geometry as geom 
		FROM %s;`, r, name)).Error
		if err != nil {
			log.Error(err)
			return 5001
		}
		return 200
	}

	err := db.Exec(fmt.Sprintf(`CREATE TABLE buffers AS 
						SELECT 机构号,名称,st_buffer(geom::geography,%f)::geometry as geom 
						FROM %s LIMIT 0;`, r, name)).Error
	if err != nil {
		log.Error(err)
		return 5001
	}
	field := cfgV.GetString("buffer.field")
	values := cfgV.GetStringSlice("buffer.values")
	scales := cfgV.GetStringSlice("buffer.scales")
	if len(scales) < len(values) {
		return 5005
	}
	for i, v := range values {
		scale, _ := strconv.ParseFloat(strings.TrimSpace(scales[i]), 32)
		if err != nil {
			log.Error(fmt.Errorf("could not parse %q to floats: %v", scales[i], err))
			return 5005
		}
		r = r * scale

		s := fmt.Sprintf(`INSERT INTO buffers 
						SELECT 机构号,名称,st_buffer(geom::geography,%f)::geometry as geom FROM %s
						WHERE %s='%s';`, r, name, field, v)

		err = db.Exec(s).Error
		if err != nil {
			log.Error(err)
			return 5001
		}
	}

	db.Exec(`DROP TABLE if EXISTS tmp_lines;`)
	err = db.Exec(`CREATE TABLE tmp_lines AS
	SELECT 机构号,geom FROM 
	(SELECT a.机构号,st_union(st_boundary(a.geom), st_union(b.geom)) as geom FROM 
	buffers as a, 
	block_lines as b 
	WHERE st_intersects(a.geom,b.geom) 
	GROUP BY a.机构号,a.geom) as lines;`).Error
	if err != nil {
		log.Error(err)
		return 5001
	}

	db.Exec(`DROP TABLE if EXISTS tmp_polys;`)
	err = db.Exec(`CREATE TABLE tmp_polys AS
	SELECT polys.机构号, (st_dump(polys.geom)).geom FROM
	(SELECT 机构号,st_polygonize(geom) as geom FROM tmp_lines
	GROUP BY 机构号) as polys
	GROUP BY polys.机构号,polys.geom;`).Error
	if err != nil {
		log.Error(err)
		return 5001
	}
	db.Exec(`DROP TABLE if EXISTS buffers_block;`)
	err = db.Exec(`CREATE TABLE buffers_block AS
	SELECT a.机构号,a.名称,st_union(b.geom) as geom FROM banks a, tmp_polys b WHERE st_intersects(a.geom,b.geom) AND a.机构号=b.机构号
	GROUP BY a.机构号,a.名称;`).Error
	if err != nil {
		log.Error(err)
		return 5001
	}
	err = db.Exec(`INSERT INTO buffers_block (机构号,名称,geom)
	SELECT b.机构号,b.名称,b.geom FROM buffers as b
	WHERE NOT EXISTS (SELECT 机构号 FROM buffers_block WHERE 机构号=b.机构号 );`).Error
	if err != nil {
		log.Error(err)
		return 5001
	}
	return 200
}

func calcM1() {

}
func calcM2() error {
	name := "m2"
	fields := cfgV.GetStringSlice("models.m2.fields")
	scales := cfgV.GetStringSlice("models.m2.scales")
	cvar := cfgV.GetString("models.m2.const")
	log.Info(cvar)
	if len(fields) < len(scales) {
		return fmt.Errorf(`model m2 config err, the field number should equal scale number`)
	}

	var cacls []string
	//cacl fields scale
	for i, fld := range fields {
		cacls = append(cacls, fmt.Sprintf(`COALESCE(%s, 0)*%s`, fld, scales[i]))
	}

	cacls = append(cacls, cvar) //add const value
	st := fmt.Sprintf(`UPDATE %s SET "总得分"=(%s);`, name, strings.Join(cacls, "+"))
	query := db.Exec(st)
	if query.Error != nil {
		return query.Error
	}

	st = fmt.Sprintf(`UPDATE %s SET "总得分"=99 WHERE "总得分">99;`, name)
	query = db.Exec(st)
	if query.Error != nil {
		return query.Error
	}
	return nil
}

func calcM3() error {
	// name := "m3"
	var tcnt int
	db.Raw(`SELECT count(*) FROM pois;`).Row().Scan(&tcnt)
	fcnt := float32(tcnt) / 100.0
	st := fmt.Sprintf(`DROP TABLE IF EXISTS p1;
	CREATE TABLE p1 AS
	SELECT b."机构号" as id, count(a.id)/%f as res FROM pois a,buffers_block b WHERE a."类型" in ('1','11') AND st_contains(b.geom,a.geom)
	GROUP BY b."机构号";
	DROP TABLE IF EXISTS p2;
	CREATE TABLE p2 AS
	SELECT b."机构号" as id, count(a.id)/%f as res FROM pois a,buffers_block b WHERE a."类型" in ('2','22') AND st_contains(b.geom,a.geom)
	GROUP BY b."机构号";
	DROP TABLE IF EXISTS p3;
	CREATE TABLE p3 AS
	SELECT b."机构号" as id, count(a.id)/%f as res FROM pois a,buffers_block b WHERE a."类型" in ('3','33') AND st_contains(b.geom,a.geom)
	GROUP BY b."机构号";
	TRUNCATE TABLE m3;
	INSERT INTO m3("机构号","商业资源")
	SELECT id, res FROM p1;
	
	UPDATE m3
	SET "对公资源"=s.res
	FROM (SELECT id, res FROM p2) AS s
	WHERE m3."机构号"=s.id;
	
	INSERT INTO m3 (机构号,"对公资源")
	SELECT id, res FROM p2 AS s
	WHERE NOT EXISTS (SELECT m3.机构号 FROM m3 WHERE m3.机构号=s.id );
		
	UPDATE m3
	SET "零售资源"=s.res
	FROM (SELECT id, res FROM p3) AS s
	WHERE m3."机构号"=s.id;
	
	INSERT INTO m3 (机构号,"零售资源")
	SELECT id, res FROM p3 AS s
	WHERE NOT EXISTS (SELECT m3.机构号 FROM m3 WHERE m3.机构号=s.id );
	
	UPDATE m3 SET "总得分"=100*(COALESCE(零售资源, 0)+COALESCE(对公资源, 0)+COALESCE(商业资源, 0));`, fcnt, fcnt, fcnt)
	query := db.Exec(st)
	if query.Error != nil {
		return query.Error
	}
	return nil
}

func calcM4() error {
	// name := "m4"
	st := fmt.Sprintf(`SELECT id,sum(w*s*cnt) FROM 
	(SELECT d.id,d.type,d.cnt,e.w FROM
	(SELECT "机构号" as id, "银行类别" as name,"网点类型" as type,COUNT(*) as cnt FROM  
	(SELECT b."机构号",a."银行类别",a."网点类型" FROM others a,buffers_block b WHERE st_contains(b.geom,a.geom) ) c
	GROUP BY c."机构号", c."银行类别",c."网点类型" ORDER BY c."机构号", c."银行类别",c."网点类型") d, m4_w e 
	WHERE d.name=e."t") f,m4_s g
	WHERE f.type=g.t
	GROUP BY id;`)
	query := db.Exec(st)
	if query.Error != nil {
		return query.Error
	}
	return nil
}
func calcM5() error {
	// name := "m5"
	st := fmt.Sprintf(`SELECT t1.name,t1.s-t2.s as result FROM
	(SELECT b.name, count(a."机构号")/110.0 as s FROM banks a,regions b WHERE st_contains(b.geom,a.geom)
	GROUP BY b.name) as t1,
	(SELECT b.name, count(a."机构号")/530.0 as s FROM others a,regions b WHERE st_contains(b.geom,a.geom) AND a."银行类别" in ('中国银行','建设银行','工商银行','农业银行','兰州农商行','甘肃银行')
	GROUP BY b.name) as t2 WHERE t1.name=t2.name
	GROUP BY t1.name,t1.s,t2.s;`)
	query := db.Exec(st)
	if query.Error != nil {
		return query.Error
	}
	return nil
}

func newFeatrue(geoType string) *geojson.Feature {
	var geometry orb.Geometry
	switch geoType {
	case "POINT":
		geometry = orb.Point{}
	case "MULTIPOINT":
		geometry = orb.MultiPoint{}
	case "LINESTRING":
		geometry = orb.LineString{}
	case "MULTILINESTRING":
		geometry = orb.MultiLineString{}
	case "POLYGON":
		geometry = orb.Polygon{}
	case "MULTIPOLYGON":
		geometry = orb.MultiPolygon{}
	default:
		return nil
	}
	return &geojson.Feature{
		Type:       "Feature",
		Geometry:   geometry,
		Properties: make(map[string]interface{}),
	}
	//test
	// var t string
	// s := fmt.Sprintf(`SELECT geometrytype(geom) FROM %s LIMIT 1;`, name)
	// err = db.Raw(s).Row().Scan(&t)
	// if err != nil {
	// 	log.Error(err)
	// 	res.Fail(c, 5001)
	// 	return
	// }
	// if newFeatrue(t) == nil {
	// 	log.Error("postgis 'geometrytype(geom)' return error")
	// 	res.Fail(c, 5001)
	// 	return
	// }
}
