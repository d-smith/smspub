package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"os"
	"log"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"github.com/aws/aws-sdk-go/service/sns"
)

var (
	subTable = os.Getenv("SUB_TABLE")
)

type AWSContext struct {
	ddbSvc dynamodbiface.DynamoDBAPI
	snsSvc snsiface.SNSAPI
}

var sender = &sns.MessageAttributeValue{
	DataType:aws.String("String"),
	StringValue:aws.String("APSStatus"),
}

var messageAttrs = map[string]*sns.MessageAttributeValue{
	"AWS.SNS.SMS.SenderID":sender,
}


func notify(awsContext *AWSContext,instanceId, state string) error {

	log.Printf("Looking for subscriptions for instance id %s\n", instanceId)

	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(instanceId),
			},
		},
		KeyConditionExpression: aws.String("InstanceID = :v1"),
		TableName:              aws.String(subTable),
	}

	result, err := awsContext.ddbSvc.Query(input)
	if err != nil {
		return err
	}

	items := result.Items
	if len(items) == 0 {
		log.Println("No subscribers to notify")
		return nil
	}

	for _, item := range items {
		notifyDestination := item["Notify"].S
		log.Printf("Notify %s\n", *notifyDestination)

		publishInput := &sns.PublishInput{
			Message: aws.String(fmt.Sprintf("new status for %s: %s", instanceId, state)),
			PhoneNumber: notifyDestination,
			MessageAttributes:messageAttrs,
		}

		_, err := awsContext.snsSvc.Publish(publishInput)
		if err != nil {
			log.Printf("Error publishing message: %s\n", err.Error())
		}
	}

	return nil
}

func makeHandler(awsContext *AWSContext) func(ctx context.Context, e events.DynamoDBEvent) {
	return func(ctx context.Context, e events.DynamoDBEvent) {

		for _, record := range e.Records {
			fmt.Printf("Processing request data for event ID %s, type %s.\n", record.EventID, record.EventName)



			instanceId := record.Change.NewImage["instanceId"].String()
			state := record.Change.NewImage["state"].String()

			err := notify(awsContext, instanceId, state)
			if err != nil {
				log.Printf("Error notifying subscribers: %s\n", err.Error())
			}
		}
	}
}



func main() {
	var awsContext AWSContext

	sess := session.New()

	awsContext.ddbSvc = dynamodb.New(sess)
	awsContext.snsSvc = sns.New(sess)

	handler := makeHandler(&awsContext)
	lambda.Start(handler)
}
