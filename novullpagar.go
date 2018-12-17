package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const tgbotToken = "PUT_BOT_TOKEN_HERE"
const tgbotLogID = -170311435
const tgbotChanID = -1001347869136
const datasource = "https://www.meneame.net/sneakme_rss?q=%23novullpagar&w=posts&h=&o=date&u="
const dynamodbRegion = "eu-west-1"
const dynamodbTable = "NoVullPagar"

var hostsAllowed = [5]string{"humblebundle.com", "www.humblebundle.com", "www.gog.com", "gog.com", "store.steampowered.com"}

// Post structure for DynamoDB post table
type Post struct {
	ID     int `json:"id"`
	PostID int `json:"PostId"`
}

// Enclosure structure for reading XML
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// Item structure for reading XML
type Item struct {
	Title     string    `xml:"title"`
	Link      string    `xml:"link"`
	Desc      string    `xml:"description"`
	GUID      string    `xml:"guid"`
	Enclosure Enclosure `xml:"enclosure"`
	PubDate   string    `xml:"pubDate"`
}

// Channel structure for reading XML
type Channel struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
	Desc  string `xml:"description"`
	Items []Item `xml:"item"`
}

// Rss structure for reading XML
type Rss struct {
	Channel Channel `xml:"channel"`
}

func tglog(message string) {
	bot, _ := tgbotapi.NewBotAPI(tgbotToken)
	msg := tgbotapi.NewMessage(tgbotLogID, fmt.Sprint(message))
	bot.Send(msg)
}

func tgsend(message string) {
	bot, _ := tgbotapi.NewBotAPI(tgbotToken)
	msg := tgbotapi.NewMessage(tgbotChanID, fmt.Sprint(message))
	bot.Send(msg)
}

func getLastPost() int {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(dynamodbRegion)},
	)
	svc := dynamodb.New(sess)
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamodbTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				N: aws.String(strconv.Itoa(1)),
			},
		},
	})
	if err != nil {
		tglog(err.Error())
	}
	post := Post{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &post)
	return post.PostID
}

func updateLastPost(postid int) {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(dynamodbRegion)},
	)
	svc := dynamodb.New(sess)
	post := Post{1, postid}
	av, err := dynamodbattribute.MarshalMap(post)
	if err != nil {
		tglog(err.Error())
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(dynamodbTable),
	}
	_, err = svc.PutItem(input)
	if err != nil {
		tglog(err.Error())
	}
}

func processPost(desc string) {
	re := regexp.MustCompile("[\"<>]")
	split := re.Split(desc, -1)
	for i := range split {
		runes := []rune(split[i])
		if string(runes[0:4]) == "http" {
			u, err := url.Parse(split[i])
			if err != nil {
				tglog(err.Error())
			}
			for _, allowed := range hostsAllowed {
				if u.Host == allowed {
					tgsend(split[i])
				}
			}
		}
	}
}

func process() {
	resp, err := http.Get(datasource)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	rss := Rss{}
	decoder := xml.NewDecoder(resp.Body)
	decoder.Strict = false
	err = decoder.Decode(&rss)

	lastPostID := getLastPost()
	newlastPostID := lastPostID
	for _, item := range rss.Channel.Items {
		params := strings.Split(item.Link, "/")
		id, _ := strconv.Atoi(params[len(params)-1])
		if id > lastPostID {
			processPost(item.Desc)

			if id > newlastPostID {
				newlastPostID = id
			}
		}
	}
	lastPostID = newlastPostID
	updateLastPost(lastPostID)
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	b := []byte(request.Body)
	var update tgbotapi.Update
	json.Unmarshal(b, &update)
	process()
	return events.APIGatewayProxyResponse{Body: request.Body, StatusCode: 200}, nil
}

func main() {
	lambda.Start(handleRequest)
}