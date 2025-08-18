package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// PostItem: DynamoDBのブログ投稿テーブルから取得するデータ構造
// TTLフィールドはblog_postsテーブルには通常ないため、定義しないか、あっても無視される
type PostItem struct {
	ID          string   `dynamodbav:"id"`
	Title       string   `dynamodbav:"title"`
	Date 		string   `dynamodbav:"date"`
	Content     string   `dynamodbav:"content"`
	Tags        []string `dynamodbav:"tags"`
	IsPublished bool     `dynamodbav:"is_published"`
}

var dbClient *dynamodb.Client
var postsTableName = os.Getenv("POSTS_TABLE_NAME") // 投稿テーブル名

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
	}
	dbClient = dynamodb.NewFromConfig(cfg)
}

// Handler handles the API Gateway proxy request to get all published blog posts.
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("Received request for get posts handler.")

	// ScanInputを作成し、DynamoDBテーブルをスキャン
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(postsTableName),
	}

	// DynamoDBから全アイテムを取得
	// Scan操作はテーブルサイズが大きくなるとパフォーマンスに影響するため、
	// 大規模なアプリケーションではQueryやGlobal Secondary Index (GSI) の利用を検討します。
	result, err := dbClient.Scan(ctx, scanInput)
	if err != nil {
		fmt.Printf("Error scanning DynamoDB table: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to scan posts: %v", err)}, nil
	}

	// 取得したアイテムをGoのPostItem構造体のスライスに変換
	var posts []PostItem
	err = attributevalue.UnmarshalListOfMaps(result.Items, &posts)
	if err != nil {
		fmt.Printf("Error unmarshalling items: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to unmarshal posts: %v", err)}, nil
	}

	// レスポンスボディをJSONに変換
	responseBody, err := json.Marshal(posts)
	if err != nil {
		fmt.Printf("Error marshalling response body: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to marshal response"}, nil
	}

	// 成功レスポンスを返す
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
