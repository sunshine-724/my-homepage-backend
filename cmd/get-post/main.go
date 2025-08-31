package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type RequestBody struct {
	ID string `json:"id"`
}

type PostItem struct {
    ID          string   `json:"id" dynamodbav:"id"`
    Title       string   `json:"title" dynamodbav:"title"`
    Date        string   `json:"date" dynamodbav:"date"`
    Content     string   `json:"content" dynamodbav:"content"`
    Tags        []string `json:"tags" dynamodbav:"tags"`
    IsPublished bool     `json:"isPublished" dynamodbav:"isPublished"`
    TTL         int64    `json:"ttl" dynamodbav:"ttl"`
}


var dbClient *dynamodb.Client
var getTableName = os.Getenv("GET_TABLE_NAME")

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
	}
	dbClient = dynamodb.NewFromConfig(cfg)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("Received request for get posts handler.")

	id := request.PathParameters["id"]

	if(id == "") {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Invalid request Body"}, nil
	}

	// キーを定義
	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: id},
	}

	// GetItemの構造体を作成
	input := &dynamodb.GetItemInput{
		TableName: &getTableName,
		Key:       key,
	}

	if v, ok := input.Key["id"].(*types.AttributeValueMemberS); ok {
		fmt.Printf("TableName: %s, Key: %s\n", *input.TableName, v.Value)
	} else {
		fmt.Printf("TableName: %s, Key: (not a string)\n", *input.TableName)
	}

	result, err := dbClient.GetItem(ctx, input)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "アイテムの取得に失敗しました"}, nil
	}

	if result.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: 404, Body: "指定された主キーを持つアイテムは見つかりませんでした\n"}, nil
	}

	var postItem PostItem
	err = attributevalue.UnmarshalMap(result.Item, &postItem) // parse
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "アイテムのパースに失敗しました"}, nil
	}

	responseBody, err := json.Marshal(postItem)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "レスポンスボディの作成に失敗しました"}, nil
	}

	fmt.Println("Response Body: " + string(responseBody))

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
