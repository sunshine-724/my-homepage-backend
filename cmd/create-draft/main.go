package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
)

// RequestBody: フロントエンドから送られてくるリクエストボディ
type RequestBody struct {
	Title       string   `json:"title"`
	Date        string   `json:"date"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	IsPublished bool     `json:"isPublished"`
}

// DraftItem: DynamoDBに保存するデータ構造
type DraftItem struct {
	ID          string   `dynamodbav:"id"` // DynamoDB に送るキー名を明示
	Title       string   `dynamodbav:"title"`
	Date        string   `dynamodbav:"date"`
	Content     string   `dynamodbav:"content"`
	Tags        []string `dynamodbav:"tags"`
	IsPublished bool     `dynamodbav:"isPublished"`
	TTL         int64    `dynamodbav:"ttl"`
}

var dbClient *dynamodb.Client
var tableName = os.Getenv("TABLE_NAME")

func init() {
	// v2ではconfig.LoadDefaultConfigを使って設定をロード
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		// エラー処理
	}
	// DynamoDBクライアントをv2で作成
	dbClient = dynamodb.NewFromConfig(cfg)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 取得した値をもう一度ログ出力
	fmt.Println("TABLE_NAME from env:", tableName)

	var reqBody RequestBody
	fmt.Println("reqBody:" + request.Body)
	err := json.Unmarshal([]byte(request.Body), &reqBody)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Invalid request body"}, nil
	}

	draftID := uuid.New().String()
    fmt.Println("Generated draftID:", draftID)
	ttl := time.Now().Add(7 * 24 * time.Hour).Unix()

	item := DraftItem{
		ID:          draftID,
		Title:       reqBody.Title,
		Date:        reqBody.Date,
		Content:     reqBody.Content,
		Tags:        reqBody.Tags,
		IsPublished: false,
		TTL:         ttl,
	}

    fmt.Println("DraftItem.ID:", item.ID)

	av, err := attributevalue.MarshalMap(item)

	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to marshal item: %v", err)}, nil
	}

	// input変数に値を入れる直前のTableNameの値をログ出力
	fmt.Println("TableName passed to PutItemInput:", tableName)

	// v2のPutItemInputを作成し、PutItemを実行
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

    fmt.Println("About to save item with ID:", item.ID)

	_, err = dbClient.PutItem(ctx, input)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to put item to DynamoDB: %v", err)}, nil
	}

    fmt.Println("Saved draftID to DynamoDB:", draftID)


	responseBody, _ := json.Marshal(map[string]string{"id": draftID})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
