package edge

import (
	"errors"

	"github.com/influxdata/kapacitor/expvar"
	"github.com/influxdata/kapacitor/models"
)

type GroupedConsumer struct {
	consumer    *Consumer
	gr          GroupedReceiver
	groups      map[models.GroupID]Receiver
	current     Receiver
	Cardinality *expvar.Int
}

func NewGroupedConsumer(edge Edge, gr GroupedReceiver) *GroupedConsumer {
	gc := &GroupedConsumer{
		gr:          gr,
		groups:      make(map[models.GroupID]Receiver),
		Cardinality: new(expvar.Int),
	}
	gc.consumer = NewConsumerWithReceiver(edge, gc)
	return gc
}

func (c *GroupedConsumer) Run() error {
	return c.consumer.Run()
}

func (c *GroupedConsumer) getOrCreateGroup(group models.GroupID) Receiver {
	r, ok := c.groups[group]
	if !ok {
		c.Cardinality.Add(1)
		r = c.gr.NewGroup(group)
		c.groups[group] = r
	}
	return r
}

func (c *GroupedConsumer) BeginBatch(begin BeginBatchMessage) error {
	r := c.getOrCreateGroup(begin.Group)
	c.current = r
	return r.BeginBatch(begin)
}

func (c *GroupedConsumer) BatchPoint(p BatchPointMessage) error {
	if c.current == nil {
		return errors.New("received batch point without batch")
	}
	return c.current.BatchPoint(p)
}

func (c *GroupedConsumer) EndBatch(end EndBatchMessage) error {
	err := c.current.EndBatch(end)
	c.current = nil
	return err
}

func (c *GroupedConsumer) Point(p PointMessage) error {
	r := c.getOrCreateGroup(p.Group)
	return r.Point(p)
}

func (c *GroupedConsumer) Barrier(b BarrierMessage) error {
	// Barriers messages apply to all gorups
	for _, r := range c.groups {
		if err := r.Barrier(b); err != nil {
			return err
		}
	}
	return nil
}

type GroupedReceiver interface {
	NewGroup(group models.GroupID) Receiver
	DeleteGroup(group models.GroupID)
}
