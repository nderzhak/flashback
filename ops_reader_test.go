package flashback

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	. "gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

var (
	logger *Logger
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	TestingT(t)
}

type TestFileByLineOpsReaderSuite struct{}

var _ = Suite(&TestFileByLineOpsReaderSuite{})

// Verify if the items being read are as expected.
func CheckOpsReader(c *C, loader OpsReader) {
	expectedOpsRead := 0
	const startingTs = 1396456709420
	for op := loader.Next(); op != nil; op = loader.Next() {
		expectedOpsRead += 1
		c.Assert(op, Not(Equals), nil)
		c.Assert(loader.OpsRead(), Equals, expectedOpsRead)

		// check the "ts" field
		CheckTime(c, float64(startingTs+loader.OpsRead()), op.Timestamp)

		// check the "o" field
		var content = op.Content["o"].(map[string]interface{})
		// certain key exists
		for i := 1; i <= 5; i++ {
			logTypeKey := fmt.Sprintf("logType%d", i)
			logType := content[logTypeKey]
			if i != expectedOpsRead {
				c.Assert(logType, IsNil)
			} else {
				c.Assert(logType, NotNil)
			}
		}
		// check the value for the shared key
		message := fmt.Sprintf("m%d", expectedOpsRead)
		c.Assert(message, Equals, content["message"].(string))
	}
	c.Assert(expectedOpsRead, Equals, 5)
}

func CheckSkipOps(c *C, loader OpsReader) {
	expectedOpsRead := 0

	// Skip a single op
	loader.SkipOps(1)

	// Read all remaining ops
	for op := loader.Next(); op != nil; op = loader.Next() {
		expectedOpsRead += 1
		c.Assert(op, Not(Equals), nil)
		c.Assert(loader.OpsRead(), Equals, expectedOpsRead)
	}

	// Verify that only 4 ops are read, since we skipped one
	c.Assert(expectedOpsRead, Equals, 4)
}

func CheckSetStartTime(c *C, loader OpsReader) {
	expectedOpsRead := 0
	numSkipped, err := loader.SetStartTime(1396456709424)
	c.Assert(err, Equals, nil)
	c.Assert(numSkipped, Equals, int64(4))

	for op := loader.Next(); op != nil; op = loader.Next() {
		expectedOpsRead += 1
		c.Assert(op, Not(Equals), nil)
		c.Assert(loader.OpsRead(), Equals, expectedOpsRead)
	}

	// Verify that only 4 ops are read, since we skipped one
	c.Assert(expectedOpsRead, Equals, 1)
}

func (s *TestFileByLineOpsReaderSuite) TestPruneEmptyUpdateObj(c *C) {
	// Check findAndModify and update structures to ensure nil $unsets are removed
	testJsonString := `{"query": {"$or": [{"_acl": {"$exists": false}}, {"_acl.*.w": true}], "_id": "YDHJwP5hFX"}, "updateobj": {"$set": {"_updated_at": {"$date": 1396457119032}}, "$unset": {}}, "ns": "appdata66.app_0939ec2a-b247-4485-b741-bfe069791305:Prize", "op": "update", "ts": {"$date": 1396457119032}}
		{"query": {"$or": [{"_acl": {"$exists": false}}, {"_acl.*.w": true}], "_id": "YDHJwP5hFX"}, "updateobj": {"$set": {"_updated_at": {"$date": 1396457119032}}, "$unset": {}}, "ns": "appdata66.app_0939ec2a-b247-4485-b741-bfe069791305:Prize", "op": "update", "ts": {"$date": 1396457119032}}`
	reader := bytes.NewReader([]byte(testJsonString))
	err, loader := NewByLineOpsReader(reader, logger, "")
	c.Assert(err, Equals, nil)

	for op := loader.Next(); op != nil; op = loader.Next() {
		doc := op.Content
		var updateMap map[string]interface{}
		if op.Type == "command" {
			updateMap = doc["command"].(map[string]interface{})["update"].(map[string]interface{})
		} else if op.Type == "update" {
			updateMap = doc["updateobj"].(map[string]interface{})
		}
		c.Assert(updateMap["$unset"], Equals, nil)
	}
}

func (s *TestFileByLineOpsReaderSuite) TestExtendedJSONParsing(c *C) {
	testJsonString := `{"ns":"foo.bar","ntoreturn":-1,"ntoskip":0,"op":"query","query":{"a":"a1"},"ts":{"$date":"1970-01-17T13:27:30.409Z"}}
	{"command":{"ping":NumberInt(1),"timeout":NumberInt(1000000)},"ns":"admin.$cmd","ntoreturn":-1,"ntoskip":0,"op":"command","ts":{"$date":"1970-01-17T13:27:30.409Z"}}
	{"ns":"foo.bar","ntoreturn":100,"ntoskip":0,"op":"query","query":{"$maxScan":NumberInt(500000),"$query":{"a":"a1","b":{"$in":[null,"b1","b2"]}}},"ts":{"$date":"1970-01-17T13:27:30.409Z"}}
	`
	reader := bytes.NewReader([]byte(testJsonString))
	err, loader := NewByLineOpsReader(reader, logger, "")
	c.Assert(err, Equals, nil)

	for op := loader.Next(); op != nil; op = loader.Next() {
	}

	c.Assert(loader.OpsRead(), Equals, 3)
}

func (s *TestFileByLineOpsReaderSuite) TestFileByLineOpsReader(c *C) {
	logger, _ = NewLogger("", "")

	testJsonString :=
		`{ "ts": {"$date" : 1396456709421}, "ns": "db.coll", "op": "insert", "o": {"logType1": "warning", "message": "m1"} }
        { "ts": {"$date": 1396456709422}, "ns": "db.coll", "op": "insert", "o": {"logType2": "warning", "message": "m2"} }
        { "ts": {"$date": 1396456709423}, "ns": "db.coll", "op": "insert", "o": {"logType3": "warning", "message": "m3"} }
        { "ts": {"$date": 1396456709424}, "ns": "db.coll", "op": "insert", "o": {"logType4": "warning", "message": "m4"} }
        { "ts": {"$date": 1396456709425}, "ns": "db.coll", "op": "insert", "o": {"logType5": "warning", "message": "m5"} }`
	reader := bytes.NewReader([]byte(testJsonString))
	err, loader := NewByLineOpsReader(reader, logger, "")
	c.Assert(err, Equals, nil)
	CheckOpsReader(c, loader)

	// Reset the reader so that we can test SkipOps
	reader = bytes.NewReader([]byte(testJsonString))
	err, loader = NewByLineOpsReader(reader, logger, "")
	c.Assert(err, Equals, nil)
	CheckSkipOps(c, loader)

	// Reset the reader so that we can test SetStartTime
	reader = bytes.NewReader([]byte(testJsonString))
	err, loader = NewByLineOpsReader(reader, logger, "")
	c.Assert(err, Equals, nil)
	CheckSetStartTime(c, loader)
}

func (s *TestFileByLineOpsReaderSuite) TestOpFilter(c *C) {
	logger, _ = NewLogger("", "")
	fmt.Println("opfilter")

	testJsonString :=
		`{ "ts": {"$date" : 1396456709421}, "ns": "db.coll", "op": "insert", "o": {"logType1": "warning", "message": "m1"} }
		 {"query": {"$or": [{"_acl": {"$exists": false}}, {"_acl.*.w": true}], "_id": "YDHJwP5hFX"}, "updateobj": {"$set": {"_updated_at": {"$date": 1396457119032}}, "$unset": {}}, "ns": "appdata66.app_0939ec2a-b247-4485-b741-bfe069791305:Prize", "op": "update", "ts": {"$date": 1396457119032}}
		 {"ns": "lala.la", "command": {"query": {"_id": "IamId"}, "findAndModify": "app_2", "update": {"$set": {"_updated_at": {"$date": 1424226458004}}}}, "ts": {"$date": 1424226458010}, "op": "command"}`

	test := func(opFilter string, expectedOps int) {
		reader := bytes.NewReader([]byte(testJsonString))
		err, loader := NewByLineOpsReader(reader, logger, opFilter)
		c.Assert(err, Equals, nil)
		opsRead := 0
		for op := loader.Next(); op != nil; op = loader.Next() {
			opsRead += 1
		}
		c.Assert(opsRead, Equals, expectedOps)
	}

	test("", 3)
	test("update", 1)
	test("update,insert", 2)
	test("update,insert,command", 3)
}

func CheckTime(c *C, pythonTime float64, goTime time.Time) {
	c.Assert(goTime.Unix(), Equals, int64(pythonTime)/1e3)
	c.Assert(goTime.UnixNano(), Equals, int64(pythonTime)*1e6)
}

func (s *TestFileByLineOpsReaderSuite) TestSimplenormalizeObj(c *C) {
	ts := 1396510695969.0
	objWithTime := map[string]interface{}{
		"ts": map[string]interface{}{"$date": ts},
	}
	normalizeObj(objWithTime)
	CheckTime(c, ts, objWithTime["ts"].(time.Time))
}

func (s *TestFileByLineOpsReaderSuite) TestComplexnormalizeObj(c *C) {
	complicatedItem := map[string]interface{}{
		"ts": map[string]interface{}{"$date": 1388810695888.0},
		"doc1": map[string]interface{}{
			"doc2": map[string]interface{}{"$date": 1388810611111.0},
		},
		"doc3": []interface{}{
			map[string]interface{}{
				"doc4": map[string]interface{}{
					"$date": 1388810622222.0,
				},
			},
		},
		"doc5": map[string]interface{}{
			"$oid": "533c3d03c23fffd217678ee7",
		},
		"doc6": map[string]interface{}{
			"$binary": "dmFsaWRiYXNlNjQ=",
			"$type":   "00",
		},
		"doc7": map[string]interface{}{
			"$regex":   "abc",
			"$options": "gims",
		},
		"doc8": map[string]interface{}{
			"$minKey": "1",
		},
		"doc9": map[string]interface{}{
			"$maxKey": "1",
		},
		"doc10": map[string]interface{}{
			"$undefined": "true",
		},
	}
	normalizeObj(complicatedItem)
	// check the ts
	CheckTime(c, 1388810695888.0, complicatedItem["ts"].(time.Time))

	// check ts inside map(s)
	doc1 := complicatedItem["doc1"].(map[string]interface{})
	ts1 := doc1["doc2"].(time.Time)
	CheckTime(c, 1388810611111.0, ts1)

	// check ts inside list(s)
	doc3 := complicatedItem["doc3"].([]interface{})
	ts2 := doc3[0].(map[string]interface{})["doc4"].(time.Time)
	CheckTime(c, 1388810622222.0, ts2)

	// check the id
	doc5 := complicatedItem["doc5"].(bson.ObjectId)
	c.Assert(doc5, Equals, bson.ObjectIdHex("533c3d03c23fffd217678ee7"))

	// check that binary types are converted to bson.Binary
	fmt.Println(complicatedItem["doc6"])
	doc6 := complicatedItem["doc6"].(bson.Binary)
	c.Assert(doc6.Kind, Equals, byte(00))
	c.Assert(doc6.Data, DeepEquals, []byte("validbase64"))

	// check that regex types are converted to bson.RegEx
	doc7 := complicatedItem["doc7"].(bson.RegEx)
	c.Assert(doc7.Pattern, Equals, "abc")
	c.Assert(doc7.Options, Equals, "gims")

	// check minKey/maxKey
	doc8 := complicatedItem["doc8"]
	c.Assert(doc8, DeepEquals, bson.MinKey)
	// check minKey/maxKey
	doc9 := complicatedItem["doc9"]
	c.Assert(doc9, DeepEquals, bson.MaxKey)
	// check undefined
	doc10 := complicatedItem["doc10"]
	c.Assert(doc10, DeepEquals, bson.Undefined)
}
