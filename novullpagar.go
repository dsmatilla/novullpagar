package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const tgbot_token = "PUT_BOT_TOKEN_HERE"
const tgbot_log_id = -170311435
const tgbot_chan_id = -1001347869136
const datasource = "https://www.meneame.net/sneakme_rss?q=%23novullpagar&w=posts&h=&o=date&u="
const dynamodb_region = "eu-west-1"
const dynamodb_table = "NoVullPagar"

var hosts_allowed = [5]string{"humblebundle.com", "www.humblebundle.com", "www.gog.com", "gog.com", "store.steampowered.com"}

type Post struct {
	ID     int `json:"id"`
	PostId int `json:"PostId"`
}

type Enclosure struct {
	Url    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type Item struct {
	Title     string    `xml:"title"`
	Link      string    `xml:"link"`
	Desc      string    `xml:"description"`
	Guid      string    `xml:"guid"`
	Enclosure Enclosure `xml:"enclosure"`
	PubDate   string    `xml:"pubDate"`
}

type Channel struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
	Desc  string `xml:"description"`
	Items []Item `xml:"item"`
}

type Rss struct {
	Channel Channel `xml:"channel"`
}

func tglog(message string) {
	bot, _ := tgbotapi.NewBotAPI(tgbot_token)
	msg := tgbotapi.NewMessage(tgbot_log_id, fmt.Sprint(message))
	bot.Send(msg)
}

func tgsend(message string) {
	bot, _ := tgbotapi.NewBotAPI(tgbot_token)
	msg := tgbotapi.NewMessage(tgbot_chan_id, fmt.Sprint(message))
	bot.Send(msg)
}

func getLastPost() (int) {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(dynamodb_region)},
	)
	svc := dynamodb.New(sess)
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamodb_table),
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
	return post.PostId
}

func updateLastPost(postid int) {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(dynamodb_region)},
	)
	svc := dynamodb.New(sess)
	post := Post{1, postid}
	av, err := dynamodbattribute.MarshalMap(post)
	if err != nil {
		tglog(err.Error())
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(dynamodb_table),
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
		if (string(runes[0:4]) == "http") {
			u, err := url.Parse(split[i])
			if err != nil {
				tglog(err.Error())
			}
			for _, allowed := range hosts_allowed {
				if (u.Host == allowed) {
					tgsend(split[i])
				}
			}
		}
	}
}

func process() {
	resp, err := http.Get(datasource)
	defer resp.Body.Close()
	rss := Rss{}
	decoder := xml.NewDecoder(resp.Body)
	decoder.Strict = false
	err = decoder.Decode(&rss)
	if err != nil {
		return
	}
	lastPostId := getLastPost()
	newLastPostId := lastPostId
	for _, item := range rss.Channel.Items {
		params := strings.Split(item.Link, "/")
		id, _ := strconv.Atoi(params[len(params)-1])
		if (id > lastPostId) {
			processPost(item.Desc)

			if (id > newLastPostId) {
				newLastPostId = id
			}
		}
	}
	lastPostId = newLastPostId
	updateLastPost(lastPostId)
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
