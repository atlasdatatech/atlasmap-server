package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/gin-gonic/gin"
	"github.com/shiena/ansicolor"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type route struct {
	method string
	path   string
}

var atlasAPI = []route{
	{"POST", "/sign/in/"},
	{"POST", "/sign/in/"},
	{"POST", "/sign/rest/"},
	{"GET", "/sign/verify/:user/:token/"},
	{"POST", "/sign/reset/:user/:token/"},
}

func initLoger() {
	//InitLog 初始化日志
	log.SetFormatter(&nested.Formatter{
		HideKeys:        true,
		ShowFullLevel:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		// FieldsOrder: []string{"component", "category"},
	})
	log.SetOutput(ansicolor.NewAnsiColorWriter(os.Stdout))
	log.SetLevel(log.DebugLevel)
}

func debugSetup() *gin.Engine {
	initLoger()
	initConf("conf.toml")
	var err error
	db, err = initDb()
	if err != nil {
		log.Fatalf("init db error, details: %s", err)
	}

	provd, err = initProvider()
	if err != nil {
		log.Fatalf("init provider error: %s", err)
	}

	authMid, err = initJWT()
	if err != nil {
		log.Fatalf("init jwt error: %s", err)
	}

	casEnf, err = initEnforcer()
	if err != nil {
		log.Fatalf("init enforcer error: %s", err)
	}

	initSystemUser()
	initTaskRouter()
	loadPubServices()

	return setupRouter()
}

// defer db.Close()
func TestPingRoute(t *testing.T) {
	router := debugSetup()
	w := httptest.NewRecorder()
	body := `{"name":"root","password":"1234"}`
	read := bytes.NewReader([]byte(body))
	req, _ := http.NewRequest("POST", "/sign/in/", read)
	req.Header = map[string][]string{
		"Content-Type": {"application/json"},
		// "Accept-Encoding": {"gzip, deflate"},
		// "Accept-Language": {"en-us"},
		// "Foo":             {"Bar", "two"},
	}
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
func BenchmarkPingRoute(b *testing.B) {
	router := debugSetup()
	req, err := http.NewRequest("POST", "/sign/in/", bytes.NewReader([]byte(`{"name":"root","password":"1234"}`)))
	if err != nil {
		panic(err)
	}
	req.Header = map[string][]string{
		"Content-Type": {"application/json"},
	}
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}