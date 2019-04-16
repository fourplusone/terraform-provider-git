// Package batches consolidates items into batches and processes them.
// Usually, the creation of items is considered to be fast compared to the
// final processing (like for example creating git commits vs. pushing them to
// a repository.
package batches

import "sync"

// In is the Input Type of an Item
type In interface{}

// Out is the Output Type of the Combine Function
type Out interface{}

// An Item to be processed
type Item struct {
	In  chan In
	Out chan Out
}

// The resolvedItem received its input and is ready for processing
type resolvedItem struct {
	In  In
	Out chan Out
}

// Combiner consolidates tasks
type Combiner struct {
	CombineFunc func(inputs []In) Out
	channels    (chan Item)
}

// Announce a new Item which will be processed on the next run of the combiner
func (c *Combiner) Announce() (chan<- In, <-chan Out) {

	item := Item{
		In:  make(chan In, 1),
		Out: make(chan Out, 1),
	}

	go func() { c.channels <- item }()

	return item.In, item.Out
}

// Deliver an Item
func (c *Combiner) Deliver(val In) <-chan Out {
	ch, ret := c.Announce()
	ch <- val
	return ret
}

// DeliverSync delivers an Item and waits until it's been processed
func (c *Combiner) DeliverSync(val In) Out {
	return <-c.Deliver(val)
}

// Close stops processing any new Requests
func (c *Combiner) Close() {
	close(c.channels)
}

// Process must be called once and will start immideatly processing incoming
// Items using the "CombineFunc".
func (c *Combiner) Process() {
	for {

		var items []resolvedItem
		var inputs []In

		// Collect all announced Items until the first one has fired
		resolved := collectAndResolve(c.channels)

		// Wait for all items to be collected
		for item := range resolved {
			items = append(items, item)
			inputs = append(inputs, item.In)
		}

		if len(items) == 0 {
			return
		}

		result := c.CombineFunc(inputs)

		for _, item := range items {
			go func(i resolvedItem) { i.Out <- result }(item)
		}

	}
}

func collectAndResolve(in <-chan Item) <-chan resolvedItem {
	var wg sync.WaitGroup
	var once sync.Once
	finished := make(chan bool)

	returned := make(chan resolvedItem)

	go func() {
	Loop:
		for {
			select {
			case a, more := <-in:
				if !more {
					break Loop
				}
				wg.Add(1)
				go func(a Item) {
					in, more := <-a.In
					if !more {
						wg.Done()
						return
					}
					r := resolvedItem{
						In:  in,
						Out: a.Out}
					once.Do(func() { close(finished) })
					returned <- r
					wg.Done()
				}(a)
			case <-finished:
				break Loop
			}
		}

		wg.Wait()
		close(returned)
	}()

	return returned
}
