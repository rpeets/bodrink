package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/slack-go/slack"
)

const (
	baseurl       = "http://192.168.101.21:8080/" // Pico W's API URL
	slacktoken    = "xoxb-slack-token"            // Slack API Token
	slackchannel  = "caseyboy"                    // Slack Channel where you like to get the alert.
	queryintervel = 60                            // Intervel between sensor query.
	alertintervel = 3600                          // Intervel between alerts while sensor readings are within the threshold for alert.
	maxdepth      = 10.5                          // Max Depth of the bowl while its empty.
	alertmin      = 10.0                          // Trigger alert while distance from sensor to water surface.
	alertmax      = 11.5                          // Max depth is used to avoid false alarm, in case bowl was removed from stand for refilling distance is usually more then 20cm
)

type SensorData struct {
	SensorId    int32     `json:"sensorId,omitempty"`
	Distance    float32   `json:"distance,omitempty"`
	AvgDistance float32   `json:"avgdistance,omitempty"`
	WaterLevel  float32   `json:"waterlevel,omitempty"`
	CurrentTime time.Time `json:"time,omitempty"`
}

var sensormap = map[int]*SensorData{}
var lastalert time.Time

func main() {
	for {
		QueryFunction()
		time.Sleep(queryintervel * time.Second)
	}
}

func GetHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 10,
	}
	return client
}

func QueryFunction() {
	var sensordata SensorData
	currentime := time.Now()
	sensordata.CurrentTime = currentime

	req, err := http.NewRequest("GET", baseurl, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("accept", "application/json")
	client := GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(body, &sensordata)
	if err != nil {
		panic(err)
	}

	RecordLastFiveQuery(&sensordata)
	alertdiff := currentime.Unix() - lastalert.Unix()

	if sensordata.AvgDistance > alertmin && sensordata.AvgDistance < alertmax && alertdiff > alertintervel {
		SendSlackMessage(sensordata)
		lastalert = currentime
	}
}

func RecordLastFiveQuery(sensordata *SensorData) {

	if _, ok := sensormap[4]; ok {
		sensormap[5] = sensormap[4]
	}
	if _, ok := sensormap[3]; ok {
		sensormap[4] = sensormap[3]
	}
	if _, ok := sensormap[2]; ok {
		sensormap[3] = sensormap[2]
	}
	if _, ok := sensormap[1]; ok {
		sensormap[2] = sensormap[1]
	}
	if sensordata != nil {
		sensormap[1] = sensordata
	}

	if _, ok := sensormap[5]; ok {
		sensordata.AvgDistance = ((sensormap[1].Distance + sensormap[2].Distance + sensormap[3].Distance + sensormap[4].Distance + sensormap[5].Distance) / 5)
		sensordata.WaterLevel = maxdepth - sensordata.AvgDistance
		fmt.Printf("Time: %20s  Q1: %4.2f Q2: %4.2f Q3: %4.2f Q4: %4.2f Q5: %4.2f  Avg Dist: %4.2fCM  WaterLevel: %4.2fCM  LastAlert: %-12s \n",
			sensormap[1].CurrentTime.Format("2006-01-02 15:04:05 PST"), sensormap[1].Distance, sensormap[2].Distance,
			sensormap[3].Distance, sensormap[4].Distance, sensormap[5].Distance, sensordata.AvgDistance, sensordata.WaterLevel, lastalert.Format("15:04:05 PST"))
	}
}

func SendSlackMessage(sensordata SensorData) {
	api := slack.New(slacktoken)

	hblk, fblk := formatSlackMessageC(sensordata)
	_, _, err := api.PostMessage(slackchannel,
		slack.MsgOptionBlocks(hblk, fblk),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func formatSlackMessageC(sensordata SensorData) (*slack.SectionBlock, *slack.SectionBlock) {

	headerformat := fmt.Sprintf(":dog: *Mommy Girl, I Want Water!!!*")
	headerText := slack.NewTextBlockObject("mrkdwn", headerformat, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	aField := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Time:*  %s", sensordata.CurrentTime.Format("2006-01-02 15:04:05 PST")), false, false)
	bField := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Distance:*  %.2f CM", sensordata.AvgDistance), false, false)
	cField := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Waterlevel:*  %.2f CM", sensordata.WaterLevel), false, false)

	fieldSlice := make([]*slack.TextBlockObject, 0)
	fieldSlice = append(fieldSlice, aField)
	fieldSlice = append(fieldSlice, bField)
	fieldSlice = append(fieldSlice, cField)
	fieldsSection := slack.NewSectionBlock(nil, fieldSlice, nil)

	return headerSection, fieldsSection
}
