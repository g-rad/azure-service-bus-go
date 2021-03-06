package servicebus

//	MIT License
//
//	Copyright (c) Microsoft Corporation. All rights reserved.
//
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//	The above copyright notice and this permission notice shall be included in all
//	copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//	SOFTWARE

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Azure/azure-amqp-common-go/uuid"
	"github.com/mitchellh/mapstructure"
	"pack.ag/amqp"
)

type (
	// Message is an Service Bus message to be sent or received
	Message struct {
		ContentType      string
		CorrelationID    string
		Data             []byte
		DeliveryCount    uint32
		GroupID          *string
		GroupSequence    *uint32
		ID               string
		Label            string
		ReplyTo          string
		ReplyToGroupID   string
		To               string
		TTL              *time.Duration
		LockToken        *uuid.UUID
		SystemProperties *SystemProperties
		UserProperties   map[string]interface{}
		message          *amqp.Message
	}

	// DispositionAction represents the action to notify Azure Service Bus of the Message's disposition
	DispositionAction func(ctx context.Context)

	// MessageErrorCondition represents a well-known collection of AMQP errors
	MessageErrorCondition string

	// SystemProperties are used to store properties that are set by the system.
	SystemProperties struct {
		LockedUntil            *time.Time `mapstructure:"x-opt-locked-until"`
		SequenceNumber         *int64     `mapstructure:"x-opt-sequence-number"`
		PartitionID            *int16     `mapstructure:"x-opt-partition-id"`
		PartitionKey           *string    `mapstructure:"x-opt-partition-key"`
		EnqueuedTime           *time.Time `mapstructure:"x-opt-enqueued-time"`
		DeadLetterSource       *string    `mapstructure:"x-opt-deadletter-source"`
		ScheduledEnqueueTime   *time.Time `mapstructure:"x-opt-scheduled-enqueue-time"`
		EnqueuedSequenceNumber *int64     `mapstructure:"x-opt-enqueue-sequence-number"`
		ViaPartitionKey        *string    `mapstructure:"x-opt-via-partition-key"`
	}

	mapStructureTag struct {
		Name         string
		PersistEmpty bool
	}
)

// Error Conditions
const (
	ErrorInternalError         MessageErrorCondition = "amqp:internal-error"
	ErrorNotFound              MessageErrorCondition = "amqp:not-found"
	ErrorUnauthorizedAccess    MessageErrorCondition = "amqp:unauthorized-access"
	ErrorDecodeError           MessageErrorCondition = "amqp:decode-error"
	ErrorResourceLimitExceeded MessageErrorCondition = "amqp:resource-limit-exceeded"
	ErrorNotAllowed            MessageErrorCondition = "amqp:not-allowed"
	ErrorInvalidField          MessageErrorCondition = "amqp:invalid-field"
	ErrorNotImplemented        MessageErrorCondition = "amqp:not-implemented"
	ErrorResourceLocked        MessageErrorCondition = "amqp:resource-locked"
	ErrorPreconditionFailed    MessageErrorCondition = "amqp:precondition-failed"
	ErrorResourceDeleted       MessageErrorCondition = "amqp:resource-deleted"
	ErrorIllegalState          MessageErrorCondition = "amqp:illegal-state"
)

const (
	lockTokenName = "x-opt-lock-token"
)

// NewMessageFromString builds an Message from a string message
func NewMessageFromString(message string) *Message {
	return NewMessage([]byte(message))
}

// NewMessage builds an Message from a slice of data
func NewMessage(data []byte) *Message {
	return &Message{
		Data: data,
	}
}

// Complete will notify Azure Service Bus that the message was successfully handled and should be deleted from the queue
func (m *Message) Complete() DispositionAction {
	return func(ctx context.Context) {
		span, _ := m.startSpanFromContext(ctx, "sb.Message.Complete")
		defer span.Finish()

		m.message.Accept()
	}
}

// Abandon will notify Azure Service Bus the message failed but should be re-queued for delivery.
func (m *Message) Abandon() DispositionAction {
	return func(ctx context.Context) {
		span, _ := m.startSpanFromContext(ctx, "sb.Message.Abandon")
		defer span.Finish()

		m.message.Modify(false, false, nil)
	}
}

// TODO: Defer - will move to the "defer" queue and user will need to track the sequence number
// FailButRetryElsewhere will notify Azure Service Bus the message failed but should be re-queued for deliver to any
// other link but this one.
//func (m *Message) FailButRetryElsewhere() DispositionAction {
//	return func(ctx context.Context) {
//		span, _ := m.startSpanFromContext(ctx, "sb.Message.FailButRetryElsewhere")
//		defer span.Finish()
//
//		m.message.Modify(true, true, nil)
//	}
//}

// Release will notify Azure Service Bus the message should be re-queued without failure.
//func (m *Message) Release() DispositionAction {
//	return func(ctx context.Context) {
//		span, _ := m.startSpanFromContext(ctx, "sb.Message.Release")
//		defer span.Finish()
//
//		m.message.Release()
//	}
//}

// DeadLetter will notify Azure Service Bus the message failed and should not re-queued
func (m *Message) DeadLetter(err error) DispositionAction {
	return func(ctx context.Context) {
		span, _ := m.startSpanFromContext(ctx, "sb.Message.DeadLetter")
		defer span.Finish()

		amqpErr := amqp.Error{
			Condition:   amqp.ErrorCondition(ErrorInternalError),
			Description: err.Error(),
		}
		m.message.Reject(&amqpErr)
	}
}

// DeadLetterWithInfo will notify Azure Service Bus the message failed and should not be re-queued with additional
// context
func (m *Message) DeadLetterWithInfo(err error, condition MessageErrorCondition, additionalData map[string]string) DispositionAction {
	var info map[string]interface{}
	if additionalData != nil {
		info = make(map[string]interface{}, len(additionalData))
		for key, val := range additionalData {
			info[key] = val
		}
	}

	return func(ctx context.Context) {
		span, _ := m.startSpanFromContext(ctx, "sb.Message.DeadLetterWithInfo")
		defer span.Finish()

		amqpErr := amqp.Error{
			Condition:   amqp.ErrorCondition(condition),
			Description: err.Error(),
			Info:        info,
		}
		m.message.Reject(&amqpErr)
	}
}

// ScheduleAt will ensure Azure Service Bus delivers the message after the time specified
// (usually within 1 minute after the specified time)
func (m *Message) ScheduleAt(t time.Time) {
	if m.SystemProperties == nil {
		m.SystemProperties = new(SystemProperties)
	}
	utcTime := t.UTC()
	m.SystemProperties.ScheduledEnqueueTime = &utcTime
}

// Set implements opentracing.TextMapWriter and sets properties on the event to be propagated to the message broker
func (m *Message) Set(key, value string) {
	if m.UserProperties == nil {
		m.UserProperties = make(map[string]interface{})
	}
	m.UserProperties[key] = value
}

// ForeachKey implements the opentracing.TextMapReader and gets properties on the event to be propagated from the message broker
func (m *Message) ForeachKey(handler func(key, val string) error) error {
	for key, value := range m.UserProperties {
		err := handler(key, value.(string))
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) toMsg() (*amqp.Message, error) {
	amqpMsg := m.message
	if amqpMsg == nil {
		amqpMsg = amqp.NewMessage(m.Data)
	}

	amqpMsg.Properties = &amqp.MessageProperties{
		MessageID: m.ID,
	}

	if m.GroupID != nil {
		amqpMsg.Properties.GroupID = *m.GroupID
	}

	if m.GroupSequence != nil {
		amqpMsg.Properties.GroupSequence = *m.GroupSequence
	}

	amqpMsg.Properties.CorrelationID = m.CorrelationID
	amqpMsg.Properties.ContentType = m.ContentType
	amqpMsg.Properties.Subject = m.Label
	amqpMsg.Properties.To = m.To
	amqpMsg.Properties.ReplyTo = m.ReplyTo
	amqpMsg.Properties.ReplyToGroupID = m.ReplyToGroupID

	if len(m.UserProperties) > 0 {
		amqpMsg.ApplicationProperties = make(map[string]interface{})
		for key, value := range m.UserProperties {
			amqpMsg.ApplicationProperties[key] = value
		}
	}

	if m.SystemProperties != nil {
		sysPropMap, err := encodeStructureToMap(m.SystemProperties)
		if err != nil {
			return nil, err
		}
		amqpMsg.Annotations = annotationsFromMap(sysPropMap)
	}

	if m.LockToken != nil {
		if amqpMsg.DeliveryAnnotations == nil {
			amqpMsg.DeliveryAnnotations = make(amqp.Annotations)
		}
		amqpMsg.DeliveryAnnotations[lockTokenName] = *m.LockToken
	}

	if m.TTL != nil {
		if amqpMsg.Header == nil {
			amqpMsg.Header = new(amqp.MessageHeader)
		}
		amqpMsg.Header.TTL = *m.TTL
	}

	return amqpMsg, nil
}

func annotationsFromMap(m map[string]interface{}) amqp.Annotations {
	a := make(amqp.Annotations)
	for key, val := range m {
		a[key] = val
	}
	return a
}

func messageFromAMQPMessage(msg *amqp.Message) (*Message, error) {
	return newMessage(msg.Data[0], msg)
}

func newMessage(data []byte, amqpMsg *amqp.Message) (*Message, error) {
	msg := &Message{
		Data:    data,
		message: amqpMsg,
	}

	if amqpMsg == nil {
		return msg, nil
	}

	if amqpMsg.Properties != nil {
		if id, ok := amqpMsg.Properties.MessageID.(string); ok {
			msg.ID = id
		}
		msg.GroupID = &amqpMsg.Properties.GroupID
		msg.GroupSequence = &amqpMsg.Properties.GroupSequence
		if id, ok := amqpMsg.Properties.CorrelationID.(string); ok {
			msg.CorrelationID = id
		}
		msg.ContentType = amqpMsg.Properties.ContentType
		msg.Label = amqpMsg.Properties.Subject
		msg.To = amqpMsg.Properties.To
		msg.ReplyTo = amqpMsg.Properties.ReplyTo
		msg.ReplyToGroupID = amqpMsg.Properties.ReplyToGroupID
		msg.DeliveryCount = amqpMsg.Header.DeliveryCount + 1
		msg.TTL = &amqpMsg.Header.TTL
	}

	if amqpMsg.ApplicationProperties != nil {
		msg.UserProperties = make(map[string]interface{}, len(amqpMsg.ApplicationProperties))
		for key, value := range amqpMsg.ApplicationProperties {
			msg.UserProperties[key] = value
		}
	}

	if amqpMsg.Annotations != nil {
		if err := mapstructure.Decode(amqpMsg.Annotations, &msg.SystemProperties); err != nil {
			return msg, err
		}
	}
	if amqpMsg.DeliveryTag != nil && len(amqpMsg.DeliveryTag) > 0 {
		lockToken, err := lockTokenFromMessageTag(amqpMsg)
		if err != nil {
			return msg, err
		}
		msg.LockToken = lockToken
	}

	return msg, nil
}

func lockTokenFromMessageTag(msg *amqp.Message) (*uuid.UUID, error) {
	if len(msg.DeliveryTag) != 16 {
		return nil, fmt.Errorf("the message contained an invalid delivery tag: %v", msg.DeliveryTag)
	}

	var swapIndex = func(indexOne, indexTwo int, array *[16]byte) {
		v1 := array[indexOne]
		array[indexOne] = array[indexTwo]
		array[indexTwo] = v1
	}

	// Get lock token from the deliveryTag
	var lockTokenBytes [16]byte
	copy(lockTokenBytes[:], msg.DeliveryTag[:16])
	// translate from .net guid byte serialisation format to amqp rfc standard
	swapIndex(0, 3, &lockTokenBytes)
	swapIndex(1, 2, &lockTokenBytes)
	swapIndex(4, 5, &lockTokenBytes)
	swapIndex(6, 7, &lockTokenBytes)
	amqpUUID := uuid.UUID(lockTokenBytes)

	return &amqpUUID, nil
}

func encodeStructureToMap(structPointer interface{}) (map[string]interface{}, error) {
	valueOfStruct := reflect.ValueOf(structPointer)
	s := valueOfStruct.Elem()
	if s.Kind() != reflect.Struct {
		return nil, fmt.Errorf("must provide a struct")
	}

	encoded := make(map[string]interface{})
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.IsValid() && f.CanSet() {
			tf := s.Type().Field(i)
			tag, err := parseMapStructureTag(tf.Tag)
			if err != nil {
				return nil, err
			}

			if tag != nil {
				switch f.Kind() {
				case reflect.Ptr:
					if !f.IsNil() || tag.PersistEmpty {
						if f.IsNil() {
							encoded[tag.Name] = nil
						} else {
							encoded[tag.Name] = f.Elem().Interface()
						}
					}
				default:
					if f.Interface() != reflect.Zero(f.Type()).Interface() || tag.PersistEmpty {
						encoded[tag.Name] = f.Interface()
					}
				}
			}
		}
	}

	return encoded, nil
}

func parseMapStructureTag(tag reflect.StructTag) (*mapStructureTag, error) {
	str, ok := tag.Lookup("mapstructure")
	if !ok {
		return nil, nil
	}

	mapTag := new(mapStructureTag)
	split := strings.Split(str, ",")
	mapTag.Name = strings.TrimSpace(split[0])

	if len(split) > 1 {
		for _, tagKey := range split[1:] {
			switch tagKey {
			case "persistempty":
				mapTag.PersistEmpty = true
			default:
				return nil, fmt.Errorf("key %q is not understood", tagKey)
			}
		}
	}
	return mapTag, nil
}
