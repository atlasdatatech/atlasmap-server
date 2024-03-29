package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/maptile/tilecover"

	"github.com/paulmach/orb"

	geom "github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/mvt"
	slippy "github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mapbox/tilejson"
	"github.com/go-spatial/tegola/server"

	"github.com/jinzhu/gorm"
	"github.com/paulmach/orb/geojson"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/gin-gonic/gin"
)

func listDatasets(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}

	var dss []Dataset
	tdb := db
	pub, y := c.GetQuery("public")
	if y && strings.ToLower(pub) == "true" {
		if casEnf.Enforce(uid, "list-atlas-datasets", c.Request.Method) {
			tdb = tdb.Where("owner = ? and public = ? ", ATLAS, true)
		}
	} else {
		tdb = tdb.Where("owner = ?", uid)
	}
	kw, y := c.GetQuery("keyword")
	if y {
		tdb = tdb.Where("name ~ ?", kw)
	}
	order, y := c.GetQuery("order")
	if y {
		tdb = tdb.Order(order)
	}
	total := 0
	err := tdb.Model(&Dataset{}).Count(&total).Error
	if err != nil {
		res.Fail(c, 5001)
		return
	}
	start := 0
	rows := 10
	if offset, y := c.GetQuery("start"); y {
		rs, yr := c.GetQuery("rows") //limit count defaut 10
		if yr {
			ri, err := strconv.Atoi(rs)
			if err == nil {
				rows = ri
			}
		}
		start, _ = strconv.Atoi(offset)
		tdb = tdb.Offset(start).Limit(rows)
	}
	err = tdb.Find(&dss).Count(&total).Error
	if err != nil {
		res.Fail(c, 5001)
		return
	}
	res.DoneData(c, gin.H{
		"keyword": kw,
		"order":   order,
		"start":   start,
		"rows":    rows,
		"total":   total,
		"list":    dss,
	})

	// var dts []*Dataset
	// set.D.Range(func(_, v interface{}) bool {
	// 	dts = append(dts, v.(*Dataset))
	// 	return true
	// })

	// if uid != ATLAS && "true" == c.Query("public") {
	// 	set := userSet.service(ATLAS)
	// 	if set != nil {
	// 		set.D.Range(func(_, v interface{}) bool {
	// 			dt, ok := v.(*Dataset)
	// 			if ok {
	// 				if dt.Public {
	// 					dts = append(dts, dt)
	// 				}
	// 			}
	// 			return true
	// 		})
	// 	}
	// }
	// res.DoneData(c, dts)
}

func getDatasetInfo(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	ds := userSet.dataset(uid, did)
	if ds == nil {
		log.Warnf(`getDatasetInfo, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	res.DoneData(c, ds)
}

func updateDatasetInfo(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	ds := userSet.dataset(uid, did)
	if ds == nil {
		log.Warnf(`getDatasetInfo, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	err := c.Bind(ds)
	if err != nil {
		res.Fail(c, 4001)
		return
	}
	err = ds.Update()
	if err != nil {
		log.Errorf("updateStyleInfo, update %s's style (%s) info error, details: %s", uid, did, err)
		res.FailErr(c, err)
		return
	}
	res.Done(c, "")
}

func oneClickImport(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	ds, err := saveSource(c)
	if err != nil {
		log.Warn(err)
		res.FailErr(c, err)
		return
	}
	dss, _ := loadZipSources(ds)
	loadFromSources(dss)
	var tasks []*Task
	for _, ds := range dss {
		ds.Owner = uid
		go func() {
			err := ds.Save()
			if err != nil {
				log.Error(err)
			}
		}()
		id := ShortID()
		task := &Task{
			ID:    id,
			Base:  ds.ID,
			Owner: ds.Owner,
			Name:  ds.Name,
			Type:  DSIMPORT,
			Pipe:  make(chan struct{}),
		}
		//任务队列,若队列已满,则阻塞
		taskQueue <- task
		//任务集
		taskSet.Store(task.ID, task)
		go func(ds *DataSource, task *Task) {
			//通知goroutine任务结束
			defer func(task *Task) {
				task.Pipe <- struct{}{}
			}(task)
			st := time.Now()
			err = ds.Import(task)
			log.Infof("one key import time cost: %v", time.Since(st))
			if err != nil {
				task.Status = "failed"
				task.Error = err.Error()
			} else {
				task.Progress = 100
				task.Status = "finished"
				task.Error = ""
			}
		}(ds, task)
		//结束队列，通知完成
		go func(ds *DataSource, task *Task) {
			<-task.Pipe
			<-taskQueue
			task.save()
			taskSet.Delete(task.ID)
			if task.Error == "" {
				//todo goroute 导入，以下事务需在task完成后处理
				dt := ds.toDataset()
				err = dt.UpInsert()
				if err != nil {
					log.Errorf(`dataImport, upinsert dataset info error, details: %s`, err)
					res.FailErr(c, err)
					return
				}
				err = dt.Service()
				if err == nil {
					set.D.Store(dt.ID, dt)
					casEnf.AddPolicy(USER, dt.ID, "GET")
				}
			} else {
				log.Errorf("import task failed, details: %s", err)
			}
		}(ds, task)

		tasks = append(tasks, task)
		//todo goroute 导入，以下事务需在task完成后处理
	}
	res.DoneData(c, tasks)
}

func uploadFile(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	ds, err := saveSource(c)
	if err != nil {
		log.Warn(err)
		res.FailErr(c, err)
		return
	}
	dss, _ := loadZipSources(ds)
	loadFromSources(dss)
	for _, ds := range dss {
		ds.Owner = uid
		err = ds.Save()
		if err != nil {
			log.Errorf(`uploadFiles, upinsert datafile info error, details: %s`, err)
		}
	}
	res.DoneData(c, dss)
}

func previewFile(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	encoding := strings.ToLower(c.Query("encoding"))
	switch encoding {
	case "utf-8", "gbk", "big5", "gb18030":
	default:
	}
	id := c.Param("id")
	ds := &DataSource{}
	err := db.Where("id = ?", id).First(ds).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			log.Errorf(`dataPreview, can not find datafile, id: %s`, id)
			res.FailMsg(c, "datafile not found")
			return
		}
		log.Errorf(`dataPreview, get datafile info error, details: %s`, err)
		res.Fail(c, 5001)
		return
	}
	ds.Encoding = encoding
	err = ds.LoadFrom()
	if err != nil {
		log.Error(err)
		res.FailErr(c, err)
		return
	}
	res.DoneData(c, ds)
	return
}

func importFile(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		res.Fail(c, 4043)
		return
	}
	ds := &DataSource{}
	err := c.Bind(ds)
	if err != nil {
		log.Error(err)
		res.Fail(c, 4001)
		return
	}
	if c.Param("id") != ds.ID {
		log.Warnf("id not eq")
	}
	id := ShortID()
	task := &Task{
		ID:    id,
		Base:  ds.ID,
		Name:  ds.Name,
		Owner: ds.Owner,
		Type:  DSIMPORT,
		Pipe:  make(chan struct{}),
	}
	//任务队列
	taskQueue <- task
	// select {
	// case taskQueue <- task:
	// default:
	// 	log.Warningf("task queue overflow, request refused...")
	// 	res.FailMsg(c, "task queue overflow, request refused")
	// 	return
	// }
	//任务集
	taskSet.Store(task.ID, task)
	go func(ds *DataSource, task *Task) {
		defer func(task *Task) {
			task.Pipe <- struct{}{}
		}(task)
		err = ds.Import(task)
		if err != nil {
			task.Status = "failed"
			task.Error = err.Error()
		} else {
			task.Progress = 100
			task.Status = "finished"
			task.Error = ""
		}
	}(ds, task)

	go func(ds *DataSource, task *Task) {
		<-task.Pipe
		<-taskQueue
		task.save()
		taskSet.Delete(task.ID)
		if task.Error == "" {
			//todo goroute 导入，以下事务需在task完成后处理
			dt := ds.toDataset()
			err = dt.UpInsert()
			if err != nil {
				log.Errorf(`dataImport, upinsert dataset info error, details: %s`, err)
				res.FailErr(c, err)
				return
			}
			err = dt.Service()
			if err == nil {
				set.D.Store(dt.ID, dt)
				casEnf.AddPolicy(USER, dt.ID, "GET")
			}
		} else {
			log.Errorf("import task failed, details: %s", err)
		}
	}(ds, task)

	res.DoneData(c, task)
}

//downloadDataset 下载数据集
func downloadDataset(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dt := userSet.dataset(uid, did)
	if dt == nil {
		log.Warnf(`downloadDataset, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	file, err := os.Open(dt.Path)
	if err != nil {
		log.Errorf(`downloadDataset, open %s's tileset (%s) error, details: %s ^^`, uid, did, err)
		res.FailErr(c, err)
		return
	}
	c.Header("Content-type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename= "+dt.ID+MBTILESEXT)
	io.Copy(c.Writer, file)
	return
}

func getDistinctValues(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dt := userSet.dataset(uid, did)
	if dt == nil {
		log.Warnf(`getDistinctValues, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	flds, ok := c.GetQuery("fields")
	if !ok {
		res.DoneData(c, dt.Fields)
		return
	}
	tbname := strings.ToLower(did)
	s := fmt.Sprintf(`SELECT distinct(%s) as val,count(*) as cnt FROM "%s" GROUP BY %s;`, flds, tbname, flds)
	fmt.Println(s)
	rows, err := db.Raw(s).Rows()
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	defer rows.Close()
	type ValCnt struct {
		Val string
		Cnt int
	}
	var valCnts []ValCnt
	for rows.Next() {
		var vc ValCnt
		// ScanRows scan a row into user
		db.ScanRows(rows, &vc)
		valCnts = append(valCnts, vc)
		// do something
	}
	res.DoneData(c, valCnts)
}

func getGeojson(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dt := userSet.dataset(uid, did)
	if dt == nil {
		log.Warnf(`getDistinctValues, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	fields := c.Query("fields")
	filter := c.Query("filter")
	tbname := strings.ToLower(did)
	selStr := "st_asgeojson(geom) as geom "
	if "" != fields {
		selStr = selStr + "," + fields
	}
	var whr string
	if "" != filter {
		whr = " WHERE " + filter
	}
	s := fmt.Sprintf(`SELECT %s FROM "%s" %s;`, selStr, tbname, whr)
	rows, err := db.Raw(s).Rows() // (*sql.Rows, error)
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	defer rows.Close()
	cols, err := rows.ColumnTypes()
	if err != nil {
		res.Fail(c, 5001)
		return
	}
	fc := geojson.NewFeatureCollection()
	for rows.Next() {
		// Scan needs an array of pointers to the values it is setting
		// This creates the object and sets the values correctly
		vals := make([]interface{}, len(cols))
		for i := range cols {
			vals[i] = new(sql.RawBytes)
		}
		err = rows.Scan(vals...)
		if err != nil {
			log.Error(err)
		}

		f := geojson.NewFeature(nil)

		for i, t := range cols {
			// skip nil values.
			if vals[i] == nil {
				continue
			}
			rb, ok := vals[i].(*sql.RawBytes)
			if !ok {
				log.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, t.Name())
				continue
			}

			switch t.Name() {
			case "geom":
				geom, err := geojson.UnmarshalGeometry([]byte(*rb))
				if err != nil {
					log.Errorf("UnmarshalGeometry from geojson result error, index %d column %s", i, t.Name())
					continue
				}
				f.Geometry = geom.Geometry()
			default:
				v := string(*rb)
				switch cols[i].DatabaseTypeName() {
				case "INT", "INT4":
					f.Properties[t.Name()], _ = strconv.Atoi(v)
				case "NUMERIC", "DECIMAL": //number
					f.Properties[t.Name()], _ = strconv.ParseFloat(v, 64)
				// case "BOOL":
				// case "TIMESTAMPTZ":
				// case "_VARCHAR":
				// case "TEXT", "VARCHAR", "BIGINT":
				default:
					f.Properties[t.Name()] = v
				}
			}

		}
		fc.Append(f)
	}
	var extent []byte
	stbox := fmt.Sprintf(`SELECT st_asgeojson(st_extent(geom)) as extent FROM %s %s;`, tbname, whr)
	db.Raw(stbox).Row().Scan(&extent) // (*sql.Rows, error)
	ext, err := geojson.UnmarshalGeometry(extent)
	if err == nil {
		fc.BBox = geojson.NewBBox(ext.Geometry().Bound())
	}
	gj, err := fc.MarshalJSON()
	if err != nil {
		log.Errorf("unable to MarshalJSON of featureclection.")
		res.FailMsg(c, "unable to MarshalJSON of featureclection.")
		return
	}
	c.JSON(http.StatusOK, json.RawMessage(gj))
}

func getGeojsonLite(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dt := userSet.dataset(uid, did)
	if dt == nil {
		log.Warnf(`getGeojsonLite %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	fc, err := dt.Dump2GeoJSON()
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}

	var minx, maxx, miny, maxy float64
	stbox := fmt.Sprintf(`SELECT min(minx),max(maxx),min(miny),max(maxy) FROM "rtree_%s_geom";`, strings.ToLower(dt.ID))
	err = db.Raw(stbox).Row().Scan(&minx, &maxx, &miny, &maxy) // (*sql.Rows, error)
	if err == nil {
		fc.BBox = []float64{minx, miny, maxx, maxy}
	}
	gj, err := fc.MarshalJSON()
	if err != nil {
		log.Errorf("unable to MarshalJSON of featureclection.")
		res.FailMsg(c, "unable to MarshalJSON of featureclection.")
		return
	}
	c.JSON(http.StatusOK, json.RawMessage(gj))
}

func queryGeojson(c *gin.Context) {
	res := NewRes()
	did := c.Param("id")

	var body struct {
		Geom   string `form:"geom" json:"geom"`
		Fields string `form:"fields" json:"fields"`
		Filter string `form:"filter" json:"filter"`
	}
	err := c.Bind(&body)
	if err != nil {
		res.Fail(c, 4001)
		return
	}

	selStr := "st_asgeojson(geom) as geom "
	if "" != body.Fields {
		selStr = selStr + "," + body.Fields
	}
	var whrStr string
	if body.Geom != "" {
		whrStr = fmt.Sprintf(` WHERE geom && st_geomfromgeojson('%s')`, body.Geom)
		if "" != body.Filter {
			whrStr = whrStr + " AND " + body.Filter
		}
	} else {
		if "" != body.Filter {
			whrStr = " WHERE " + body.Filter
		}
	}

	s := fmt.Sprintf(`SELECT %s FROM "%s"  %s;`, selStr, strings.ToLower(did), whrStr)
	rows, err := db.Raw(s).Rows() // (*sql.Rows, error)
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	defer rows.Close()
	cols, err := rows.ColumnTypes()
	if err != nil {
		res.Fail(c, 5001)
		return
	}
	fc := geojson.NewFeatureCollection()
	for rows.Next() {
		// Scan needs an array of pointers to the values it is setting
		// This creates the object and sets the values correctly
		vals := make([]interface{}, len(cols))
		for i := range cols {
			vals[i] = new(sql.RawBytes)
		}
		err = rows.Scan(vals...)
		if err != nil {
			log.Error(err)
		}

		f := geojson.NewFeature(nil)

		for i, t := range cols {
			// skip nil values.
			if vals[i] == nil {
				continue
			}
			rb, ok := vals[i].(*sql.RawBytes)
			if !ok {
				log.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, t.Name())
				continue
			}

			switch t.Name() {
			case "geom":
				geom, err := geojson.UnmarshalGeometry([]byte(*rb))
				if err != nil {
					log.Errorf("UnmarshalGeometry from geojson result error, index %d column %s", i, t.Name())
					continue
				}
				f.Geometry = geom.Geometry()
			default:
				v := string(*rb)
				switch cols[i].DatabaseTypeName() {
				case "INT", "INT4":
					f.Properties[t.Name()], _ = strconv.Atoi(v)
				case "NUMERIC", "DECIMAL": //number
					f.Properties[t.Name()], _ = strconv.ParseFloat(v, 64)
				// case "BOOL":
				// case "TIMESTAMPTZ":
				// case "_VARCHAR":
				// case "TEXT", "VARCHAR", "BIGINT":
				default:
					f.Properties[t.Name()] = v
				}
			}

		}
		fc.Append(f)
	}
	gj, err := fc.MarshalJSON()
	if err != nil {
		log.Errorf("unable to MarshalJSON of featureclection.")
		res.FailMsg(c, "unable to MarshalJSON of featureclection.")
		return
	}
	res.DoneData(c, json.RawMessage(gj))
}

func cubeQuery(c *gin.Context) {
	res := NewRes()
	var body struct {
		SQL string `form:"sql" json:"sql" binding:"required"`
	}
	err := c.Bind(&body)
	if err != nil {
		res.Fail(c, 4001)
		return
	}
	rows, err := db.Raw(body.SQL).Rows() // (*sql.Rows, error)
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	defer rows.Close()
	cols, err := rows.ColumnTypes()
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	var t [][]string
	for rows.Next() {
		// Scan needs an array of pointers to the values it is setting
		// This creates the object and sets the values correctly
		vals := make([]sql.RawBytes, len(cols))
		valsScer := make([]interface{}, len(vals))
		for i := range vals {
			valsScer[i] = &vals[i]
		}
		err = rows.Scan(valsScer...)
		if err != nil {
			log.Error(err)
		}
		var r []string
		for _, v := range vals {
			// skip nil values.
			if v == nil {
				continue
			}
			r = append(r, string(v))
		}
		if len(r) == 0 {
			continue
		}
		t = append(t, r)
	}
	res.DoneData(c, t)
}

func queryExec(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dt := userSet.dataset(uid, did)
	if dt == nil {
		log.Warnf(`getDistinctValues, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	var body struct {
		SQL string `form:"sql" json:"sql" binding:"required"`
	}
	err := c.Bind(&body)
	if err != nil {
		res.Fail(c, 4001)
		return
	}
	s := strings.Replace(body.SQL, "!TABLENAME!", fmt.Sprintf(` "%s" `, strings.ToLower(did)), -1)
	rows, err := db.Raw(s).Rows() // (*sql.Rows, error)
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	defer rows.Close()

	cols, _ := rows.ColumnTypes()
	var ams [][]interface{}
	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]sql.RawBytes, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			log.Error(err)
			continue
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make([]interface{}, len(cols))
		for i, col := range columns {
			// if cols[i].Name() == "geom" || cols[i].Name() == "search" {
			// 	continue
			// }
			//"NVARCHAR", "DECIMAL", "BOOL", "INT", "BIGINT".
			v := string(col)
			switch cols[i].DatabaseTypeName() {
			case "INT", "INT4":
				m[i], _ = strconv.Atoi(v)
			case "NUMERIC", "DECIMAL": //number
				m[i], _ = strconv.ParseFloat(v, 64)
			// case "BOOL":
			// case "TIMESTAMPTZ":
			// case "_VARCHAR":
			// case "TEXT", "VARCHAR", "BIGINT":
			default:
				m[i] = v
			}
		}
		// fmt.Print(m)
		ams = append(ams, m)
	}
	res.DoneData(c, ams)
}

func queryBusiness(c *gin.Context) {
	res := NewRes()
	did := c.Param("id")
	var linkTables []string
	if did != "banks" {
		res.DoneData(c, gin.H{
			did: linkTables,
		})
		return
	}
	linkTables = viper.GetStringSlice("business.banks.linked")
	res.DoneData(c, gin.H{
		did: linkTables,
	})
}

func getBuffers(c *gin.Context) {
}

func searchGeos(c *gin.Context) {
	// res := NewRes()
	searchType := c.Param("name")
	keyword := c.Query("keyword")
	var ams []map[string]interface{}

	log.Println("***********", keyword, "**************")
	if searchType != "search" || keyword == "" {
		// res.Fail(c, 4001)
		c.JSON(http.StatusOK, ams)
		return
	}
	search := func(s string, keyword string) {
		stmt, err := db.DB().Prepare(s)
		if err != nil {
			log.Error(err)
			return
		}
		defer stmt.Close()
		rows, err := stmt.Query(keyword)
		if err != nil {
			log.Error(err)
			return
		}
		defer rows.Close()

		cols, _ := rows.ColumnTypes()
		for rows.Next() {
			// Create a slice of interface{}'s to represent each column,
			// and a second slice to contain pointers to each item in the columns slice.
			columns := make([]sql.RawBytes, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}

			// Scan the result into the column pointers...
			if err := rows.Scan(columnPointers...); err != nil {
				log.Error(err)
				continue
			}

			// Create our map, and retrieve the value for each column from the pointers slice,
			// storing it in the map with the name of the column as the key.
			m := make(map[string]interface{})
			for i, col := range columns {
				if col == nil {
					continue
				}
				//"NVARCHAR", "DECIMAL", "BOOL", "INT", "BIGINT".
				v := string(col)
				switch cols[i].DatabaseTypeName() {
				case "INT", "INT4":
					m[cols[i].Name()], _ = strconv.Atoi(v)
				case "NUMERIC", "DECIMAL": //number
					m[cols[i].Name()], _ = strconv.ParseFloat(v, 64)
				// case "BOOL":
				// case "TIMESTAMPTZ":
				// case "_VARCHAR":
				// case "TEXT", "VARCHAR", "BIGINT":
				default:
					m[cols[i].Name()] = v
				}
			}
			// fmt.Print(m)
			ams = append(ams, m)
		}
	}

	st := fmt.Sprintf(`SELECT id,名称,st_asgeojson(geom) as geom FROM regions WHERE 名称 ~ $1;`)
	fmt.Println(st)
	search(st, keyword)
	if len(ams) > 10 {
		c.JSON(http.StatusOK, ams)
		return
	}
	bbox := c.Query("bbox")
	var gfilter string
	if bbox != "" {
		gfilter = fmt.Sprintf(` geom && st_makeenvelope(%s,4326) AND `, bbox)
	}
	limit := c.Query("limit")
	var limiter string
	if limit != "" {
		limiter = fmt.Sprintf(` LIMIT %s `, limit)
	}
	st = fmt.Sprintf(`SELECT id,名称,st_asgeojson(geom) as geom,s 搜索 
	FROM (SELECT id,名称,geom,unnest(search) s FROM banks) x WHERE %s s ~ $1 GROUP BY id,名称,geom,s %s;`, gfilter, limiter)
	fmt.Println(st)
	search(st, keyword)
	if len(ams) > 10 {
		c.JSON(http.StatusOK, ams)
		return
	}
	st = fmt.Sprintf(`SELECT id,名称,st_asgeojson(geom) as geom,s 搜索 
	FROM (SELECT id,名称,geom,unnest(search) s FROM others) x WHERE %s s ~ $1 GROUP BY id,名称,geom,s %s;`, gfilter, limiter)
	fmt.Println(st)
	search(st, keyword)
	if len(ams) > 10 {
		c.JSON(http.StatusOK, ams)
		return
	}
	st = fmt.Sprintf(`SELECT 名称,st_asgeojson(geom) as geom,s 搜索 
	FROM (SELECT 名称,geom,unnest(search) s FROM pois) x WHERE %s s ~ $1 GROUP BY 名称,geom,s %s;`, gfilter, limiter)
	fmt.Println(st)
	search(st, keyword)
	if len(ams) > 10 {
		c.JSON(http.StatusOK, ams)
		return
	}
	c.JSON(http.StatusOK, ams)
}

func search(c *gin.Context) {
	// res := NewRes()
	id := c.Param("id")
	tn := strings.ToLower(id)
	var ams []map[string]interface{}

	search := func(s string, keyword string) {
		stmt, err := db.DB().Prepare(s)
		if err != nil {
			log.Error(err)
			return
		}
		defer stmt.Close()
		rows, err := stmt.Query(keyword)
		if err != nil {
			log.Error(err)
			return
		}
		defer rows.Close()

		cols, _ := rows.ColumnTypes()
		for rows.Next() {
			columns := make([]sql.RawBytes, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}

			// Scan the result into the column pointers...
			if err := rows.Scan(columnPointers...); err != nil {
				log.Error(err)
				continue
			}

			// Create our map, and retrieve the value for each column from the pointers slice,
			// storing it in the map with the name of the column as the key.
			m := make(map[string]interface{})
			for i, col := range columns {
				if col == nil {
					continue
				}
				//"NVARCHAR", "DECIMAL", "BOOL", "INT", "BIGINT".
				v := string(col)
				switch cols[i].DatabaseTypeName() {
				case "INT", "INT4":
					m[cols[i].Name()], _ = strconv.Atoi(v)
				case "NUMERIC", "DECIMAL": //number
					m[cols[i].Name()], _ = strconv.ParseFloat(v, 64)
				// case "BOOL":
				// case "TIMESTAMPTZ":
				// case "_VARCHAR":
				// case "TEXT", "VARCHAR", "BIGINT":
				default:
					m[cols[i].Name()] = v
				}
			}
			ams = append(ams, m)
		}
	}

	searchType := c.Query("type")
	geom, ok := c.GetQuery("geom")
	var gfilter string
	if ok {
		gfilter = fmt.Sprintf(` geom && st_makeenvelope(%s,4326) AND `, geom)
	}

	limiter := fmt.Sprintf(` LIMIT 10 `)
	limit, ok := c.GetQuery("limit")
	if ok {
		_, err := strconv.ParseUint(limit, 10, 32)
		if err == nil {
			limiter = fmt.Sprintf(` LIMIT %s `, limit)
		}
	}

	field, ok := c.GetQuery("field")
	if !ok {
		field = "名称"
	}

	var st string
	switch searchType {
	case "0":
		st = fmt.Sprintf(`SELECT gid,"%s",st_asgeojson(geom) as geom FROM "%s" WHERE %s "%s" ~ $1 %s ;`, field, tn, gfilter, field, limiter)
	case "1", "2":
		dt := Dataset{ID: id}
		err := db.First(&dt).Error
		if err != nil {
			log.Error(err)
			c.JSON(http.StatusOK, ams)
			return
		}
		var fields []Field
		err = json.Unmarshal(dt.Fields, &fields)
		if err != nil {
			log.Error(err)
		}
		var flds []string
		for _, f := range fields {
			flds = append(flds, f.Name)
		}
		switch searchType {
		case "1":
			st = fmt.Sprintf(`SELECT gid,"%s",st_asgeojson(geom) as geom FROM "%s" WHERE %s "%s" ~ $1 %s ;`, strings.Join(flds, `","`), tn, gfilter, field, limiter)
		case "2":
			st = fmt.Sprintf(`SELECT gid,"%s",st_asgeojson(geom) as geom FROM "%s" WHERE gid = $1 ;`, strings.Join(flds, `","`), tn)
			search(st, c.Query("gid"))
			c.JSON(http.StatusOK, ams)
			return
		}
	}
	search(st, c.Query("keyword"))
	c.JSON(http.StatusOK, ams)
}

func upInsertDataset(c *gin.Context) {
	res := NewRes()
	did := c.Param("id")

	if code := checkDataset(did); code != 200 {
		res.Fail(c, code)
		return
	}

	bank := &Bank{}
	err := c.BindJSON(bank)
	if err != nil {
		log.Error(err)
		res.Fail(c, 4001)
		return
	}

	bank.Search = []string{bank.No, bank.Name, bank.Region, bank.Type, bank.Manager}

	if db.Table(did).Where("id = ?", bank.ID).First(&Bank{}).RecordNotFound() {
		db.Omit("geom").Create(bank)
	} else {
		err := db.Table(did).Where("id = ?", bank.ID).Update(bank).Error
		if err != nil {
			log.Error(err)
			res.Fail(c, 5001)
			return
		}
	}

	if bank.X < -180 || bank.X > 180 || bank.Y < -85 || bank.Y > 85 {
		log.Errorf("x, y must be reasonable values, name")
		res.FailMsg(c, "x, y must be reasonable values")
		return
	}
	stgeo := fmt.Sprintf(`UPDATE %s SET geom = ST_GeomFromText('POINT(' || x || ' ' || y || ')',4326) WHERE id=%d;`, did, bank.ID)
	result := db.Exec(stgeo)
	if result.Error != nil {
		log.Errorf("update %s create geom error:%s", did, result.Error.Error())
		res.Fail(c, 5001)
		return
	}

	res.DoneData(c, gin.H{
		"id": bank.ID,
	})
}

//deleteDatasets 删除数据集
func deleteDatasets(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	set := userSet.service(uid)
	if set == nil {
		log.Errorf(`deleteDatasets, %s's service not found ^^`, uid)
		res.Fail(c, 4043)
		return
	}
	ids := c.Param("id")
	dids := strings.Split(ids, ",")
	for _, did := range dids {
		ds := userSet.dataset(uid, did)
		if ds == nil {
			log.Errorf(`deleteDatasets, %s's dataset (%s) not found ^^`, uid, did)
			res.Fail(c, 4046)
			return
		}
		set.D.Delete(did)
		err := db.Where("id = ?", did).Delete(Dataset{}).Error
		if err != nil {
			log.Error(err)
			res.Fail(c, 5001)
			return
		}
	}
	res.Done(c, "")
}

func deleteFeatures(c *gin.Context) {
	res := NewRes()
	did := c.Param("id")

	if code := checkDataset(did); code != 200 {
		res.Fail(c, code)
		return
	}

	var body struct {
		ID string `form:"id" json:"id" binding:"required"`
	}
	err := c.Bind(&body)
	if err != nil {
		res.Fail(c, 4001)
		return
	}
	err = db.Where("id = ?", body.ID).Delete(&Bank{}).Error
	if err != nil {
		log.Errorf("delete data : %s; dataid: %s", err, body.ID)
		res.Fail(c, 5001)
		return
	}
	res.Done(c, "")
}

func createTileLayer(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`createTileLayer, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	tl, err := dts.NewTileLayer()
	if err != nil {
		res.FailErr(c, err)
		return
	}
	log.Info(tl)
	tl.UpInsert()
	res.Done(c, "")
	return
}

func getLayerTiles(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`getTileLayer, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	// lookup our Map
	placeholder, _ := strconv.ParseUint(c.Param("z"), 10, 32)
	z := uint(placeholder)
	placeholder, _ = strconv.ParseUint(c.Param("x"), 10, 32)
	x := uint(placeholder)
	yext := c.Param("y")
	ys := strings.Split(yext, ".")
	if len(ys) != 2 {
		res.Fail(c, 404)
		return
	}
	placeholder, _ = strconv.ParseUint(ys[0], 10, 32)
	y := uint(placeholder)

	if dts.tlayer == nil {
		_, err := dts.NewTileLayer()
		if err != nil {
			log.Warn(err)
			// res.FailMsg(c, "tilelayer empty")
			// return
		}
	}

	if dts.tlayer.FilterByZoom(z) {
		log.Errorf("map (%v) has no layer, at zoom %v", did, z)
		return
	}

	tile := slippy.NewTile(z, x, y)
	var pbyte []byte
	var err error
	if dts.tlayer.Provider.Std != nil {
		pbyte, err = dts.tlayer.Encode(c.Request.Context(), tile)
	} else {
		pbyte, err = dts.tlayer.MVTEncode(c.Request.Context(), tile)
	}

	if err != nil {
		switch err {
		case context.Canceled:
			// TODO: add debug logs
			return
		default:
			errMsg := fmt.Sprintf("marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(c.Writer, errMsg, http.StatusInternalServerError)
			return
		}
	}

	// mimetype for mapbox vector tiles
	// https://www.iana.org/assignments/media-types/application/vnd.mapbox-vector-tile
	c.Header("Content-Type", mvt.MimeType)
	c.Header("Content-Encoding", "gzip")
	// c.Header("Content-Type", "application/x-protobuf")
	c.Header("Content-Length", fmt.Sprintf("%d", len(pbyte)))
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(pbyte)
	// check for tile size warnings
	if len(pbyte) > server.MaxTileSize {
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %vKb", z, x, y, len(pbyte)/1024)
	}
}

func getTileLayerJSON(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`getTileLayer, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	if dts.tlayer == nil {
		_, err := dts.NewTileLayer()
		if err != nil {
			log.Errorf("create tilelayer failed : %v", err)
			// res.FailMsg(c, "tilelayer empty")
			// return
		}
		dts.Bound()
	}
	guessZoom := func(box orb.Bound, total int) int {
		z := 0
		for ; z < 20; z++ {
			tc := tilecover.BoundCount(box, maptile.Zoom(z))
			if tc > int64(total/100) {
				return z - 1
			}
		}
		return z
	}
	gz := guessZoom(dts.BBox, dts.Total)

	// zoom := (dts.tlayer.MinZoom + dts.tlayer.MaxZoom) / 2
	attr := "atlas realtime tile layer"
	tileJSON := tilejson.TileJSON{
		Attribution: &attr,
		// Bounds:      dts.tlayer.Bounds.Extent(),
		Bounds:   [4]float64{dts.BBox.Left(), dts.BBox.Bottom(), dts.BBox.Right(), dts.BBox.Top()},
		Center:   [3]float64{dts.BBox.Center().X(), dts.BBox.Center().Y(), float64(gz)},
		Format:   "pbf",
		Name:     &dts.Name,
		Scheme:   tilejson.SchemeXYZ,
		TileJSON: tilejson.Version,
		Version:  "2",
		Grids:    make([]string, 0),
		Data:     make([]string, 0),
	}

	tileJSON.MinZoom = dts.tlayer.MinZoom
	// minzoom := uint(gz) - 3
	// if minzoom >= tileJSON.MinZoom {
	// 	tileJSON.MinZoom = minzoom
	// }
	tileJSON.MaxZoom = dts.tlayer.MaxZoom
	//	build our vector layer details
	layer := tilejson.VectorLayer{
		Version: 2,
		Extent:  4096,
		ID:      dts.tlayer.MVTName(),
		Name:    dts.tlayer.MVTName(),
		MinZoom: dts.tlayer.MinZoom,
		MaxZoom: dts.tlayer.MaxZoom,
		Tiles: []string{
			fmt.Sprintf("atlasdata://datasets/x/%s/{z}/{x}/{y}.pbf", dts.tlayer.MVTName()),
		},
	}

	switch dts.tlayer.GeomType.(type) {
	case geom.Point, geom.MultiPoint:
		layer.GeometryType = tilejson.GeomTypePoint
	case geom.Line, geom.LineString, geom.MultiLineString:
		layer.GeometryType = tilejson.GeomTypeLine
	case geom.Polygon, geom.MultiPolygon:
		layer.GeometryType = tilejson.GeomTypePolygon
	default:
		layer.GeometryType = tilejson.GeomTypeUnknown
		// TODO: debug log
	}

	// add our layer to our tile layer response
	tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)

	tileURL := fmt.Sprintf("atlasdata://datasets/x/%s/{z}/{x}/{y}.pbf", did)

	// build our URL scheme for the tile grid
	tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

	// content type
	c.Header("Content-Type", "application/json")

	// cache control headers (no-cache)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	if err := json.NewEncoder(c.Writer).Encode(tileJSON); err != nil {
		log.Printf("error encoding tileJSON for layer (%v)", did)
	}
}

func createTileMap(c *gin.Context) {
}

func getTileMap(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	log.Info(uid)
	did := c.Param("id")
	// ds := userSet.dataset(uid, did)
	// if ds == nil {
	// 	res.Fail(c, 4046)
	// 	return
	// }
	// lookup our Map
	placeholder, _ := strconv.ParseUint(c.Param("z"), 10, 32)
	z := uint(placeholder)
	placeholder, _ = strconv.ParseUint(c.Param("x"), 10, 32)
	x := uint(placeholder)
	ypbf := c.Param("y")
	ys := strings.Split(ypbf, ".")
	if len(ys) != 2 {
		res.Fail(c, 404)
		return
	}
	placeholder, _ = strconv.ParseUint(ys[0], 10, 32)
	y := uint(placeholder)
	type A struct {
		*atlas.Atlas
	}
	a := A{}
	m, err := a.Map(did)
	if err != nil {
		errMsg := fmt.Sprintf("map (%v) not configured. check your config file", did)
		log.Errorf(errMsg)
		http.Error(c.Writer, errMsg, http.StatusNotFound)
		return
	}

	// filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(z)
	if len(m.Layers) == 0 {
		log.Errorf("map (%v) has no layers, at zoom %v", did, z)
		return
	}
	ids := c.Query("layers")
	if ids != "" {
		m = m.FilterLayersByID(ids)
		if len(m.Layers) == 0 {
			log.Errorf("map (%v) has no layers, for LayerName %v at zoom %v", did, ids, z)
			return
		}
	}

	tile := slippy.NewTile(z, x, y)

	// check for the debug query string
	if true {
		m = m.AddDebugLayers()
	}

	pbyte, err := m.Encode(c.Request.Context(), tile)
	if err != nil {
		switch err {
		case context.Canceled:
			// TODO: add debug logs
			return
		default:
			errMsg := fmt.Sprintf("error marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(c.Writer, errMsg, http.StatusInternalServerError)
			return
		}
	}

	// mimetype for mapbox vector tiles
	// https://www.iana.org/assignments/media-types/application/vnd.mapbox-vector-tile
	c.Header("Content-Type", mvt.MimeType)
	c.Header("Content-Encoding", "gzip")
	// c.Header("Content-Type", "application/x-protobuf")
	c.Header("Content-Length", fmt.Sprintf("%d", len(pbyte)))
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(pbyte)
	log.Info(len(pbyte))
	// check for tile size warnings
	if len(pbyte) > server.MaxTileSize {
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %vKb", z, x, y, len(pbyte)/1024)
	}
}

func viewDataset(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`viewDataset, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	lrt := "circle"
	gt, _ := dts.GeoType()
	switch gt {
	case "Point", "MultiPoint":
		lrt = "circle"
	case "LineString", "MultiLineString":
		lrt = "line"
	case "Polygon", "MultiPolygon":
		lrt = "fill"
	default:
		t, y := c.GetQuery("type")
		if y {
			lrt = t
		}
	}
	tiles := fmt.Sprintf(`%s/datasets/x/%s/{z}/{x}/{y}.pbf`, rootURL(c.Request), did) //need use user own service set//
	c.HTML(http.StatusOK, "dataset.html", gin.H{
		"Title":     "数据集预览(T)",
		"Name":      dts.Name + "@" + dts.ID,
		"LayerName": dts.ID,
		"LayerType": lrt,
		"Format":    "pbf",
		"URL":       tiles,
	})
	return
}

func getLayerMBTiles(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`getLayerMBTiles, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	attr := "atlas realtime tile layer"
	minzoom, err := strconv.Atoi(c.Param("min"))
	if err != nil {
		log.Println("minzoom error ")
	}
	maxzoom, err := strconv.Atoi(c.Param("max"))
	if err != nil {
		log.Println("minzoom error ")
	}
	tileJSON := tilejson.TileJSON{
		Attribution: &attr,
		// Bounds:      dts.tlayer.Bounds.Extent(),
		Bounds:   [4]float64{dts.BBox.Left(), dts.BBox.Bottom(), dts.BBox.Right(), dts.BBox.Top()},
		Center:   [3]float64{dts.BBox.Center().X(), dts.BBox.Center().Y(), float64(maxzoom)},
		Format:   "pbf",
		Name:     &dts.Name,
		Scheme:   tilejson.SchemeXYZ,
		TileJSON: tilejson.Version,
		Version:  "2",
		Grids:    make([]string, 0),
		Data:     make([]string, 0),
	}

	tileJSON.MinZoom = uint(minzoom)
	tileJSON.MaxZoom = uint(maxzoom)
	//	build our vector layer details
	layer := tilejson.VectorLayer{
		Version: 2,
		ID:      dts.Name,
		Name:    dts.Name,
		MinZoom: uint(minzoom),
		MaxZoom: uint(maxzoom),
	}

	// add our layer to our tile layer response
	tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)

	tileURL := fmt.Sprintf("atlasdata://datasets/x/%s/{z}/{x}/{y}.pbf", did)

	// build our URL scheme for the tile grid
	tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

	// content type
	c.Header("Content-Type", "application/json")

	// cache control headers (no-cache)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	if err := json.NewEncoder(c.Writer).Encode(tileJSON); err != nil {
		log.Printf("error encoding tileJSON for layer (%v)", did)
	}
}

func publishToMBTiles(c *gin.Context) {
	res := NewRes()
	uid := c.GetString(userKey)
	if uid == "" {
		uid = c.GetString(identityKey)
	}
	did := c.Param("id")
	dts := userSet.dataset(uid, did)
	if dts == nil {
		log.Warnf(`publishToMBTiles, %s's dataset (%s) not found ^^`, uid, did)
		res.Fail(c, 4046)
		return
	}
	//dump geojson
	pathFile := dts.Name + ".mbtiles"
	err := dts.GeoJSON2MBTiles(pathFile, dts.Name, true)
	if err != nil {
		log.Error(err)
		res.Fail(c, 5001)
		return
	}
	res.DoneData(c, gin.H{
		"file": pathFile,
	})
}

func dts2tsLite(task *Task, dts *Dataset) (*Tileset, error) {
	s := time.Now()
	task.Progress = 20
	outfile := filepath.Join(viper.GetString("paths.tilesets"), task.Owner, dts.ID+MBTILESEXT)
	err := dts.GeoJSON2MBTiles(outfile, dts.Name, true)
	if err != nil {
		return nil, err
	}
	log.Infof("publish %v data source to tilesets(%s), takes: %v", dts.Name, outfile, time.Since(s))
	s = time.Now()

	ds := &DataSource{
		ID:    task.Base,
		Name:  task.Name,
		Owner: task.Owner,
		Path:  outfile,
	}
	//加载mbtiles
	ts, err := LoadTileset(ds)
	if err != nil {
		log.Errorf("publishTileset, could not load the new tileset %s, details: %s", outfile, err)
		return nil, err
	}
	log.Infof("load tilesets(%s), takes: %v", outfile, time.Since(s))
	return ts, nil
}
