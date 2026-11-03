package relay

import (
	"sync"
	"testing"
	"time"
)

const (
	channelReliableUnordered   = 1
	channelUnreliableOrdered   = 2
	channelUnreliableUnordered = 3
)

func sequenceRandom(values ...float64) func() float64 {
	index := 0
	return func() float64 {
		value := values[index%len(values)]
		index++
		return value
	}
}

func newTestLink(profile Profile, random func() float64) *link {
	testLink := newLink(profile)
	testLink.random = random
	return testLink
}

func TestProfileValidate(t *testing.T) {
	valid := []Profile{{}, {LatencyMs: 5000, JitterMs: 5000, LossPercent: 100}}
	for _, profile := range valid {
		if err := profile.Validate(); err != nil {
			t.Errorf("%+v should be valid: %v", profile, err)
		}
	}
	invalid := []Profile{
		{LatencyMs: -1},
		{LatencyMs: 5001},
		{JitterMs: -1},
		{JitterMs: 5001},
		{LossPercent: -0.1},
		{LossPercent: 100.1},
	}
	for _, profile := range invalid {
		if err := profile.Validate(); err == nil {
			t.Errorf("%+v should be invalid", profile)
		}
	}
}

func TestPlanAppliesLatencyAndJitterWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	// random()*2-1 is -1 for 0 and +1 for 1, the two ends of the jitter window.
	testLink := newTestLink(Profile{LatencyMs: 100, JitterMs: 30}, sequenceRandom(0, 1))

	early, dropped := testLink.plan(channelUnreliableUnordered, now)
	if dropped || early.Sub(now) != 70*time.Millisecond {
		t.Fatalf("early release = %v (dropped %v), want 70ms", early.Sub(now), dropped)
	}
	late, dropped := testLink.plan(channelUnreliableUnordered, now)
	if dropped || late.Sub(now) != 130*time.Millisecond {
		t.Fatalf("late release = %v (dropped %v), want 130ms", late.Sub(now), dropped)
	}
}

func TestPlanFloorsNegativeDelayAtZero(t *testing.T) {
	now := time.Unix(1000, 0)
	testLink := newTestLink(Profile{LatencyMs: 5, JitterMs: 50}, sequenceRandom(0))
	release, _ := testLink.plan(channelUnreliableUnordered, now)
	if !release.Equal(now) {
		t.Fatalf("release = %v, want now", release.Sub(now))
	}
}

func TestPlanDropsUnreliableFramesOnLoss(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, channel := range []int{channelUnreliableOrdered, channelUnreliableUnordered} {
		// 0.0 * 100 < 50: the loss roll hits.
		testLink := newTestLink(Profile{LatencyMs: 10, LossPercent: 50}, sequenceRandom(0.0))
		if _, dropped := testLink.plan(channel, now); !dropped {
			t.Errorf("channel %d should drop on a hit loss roll", channel)
		}
		testLink = newTestLink(Profile{LatencyMs: 10, LossPercent: 50}, sequenceRandom(0.9))
		if _, dropped := testLink.plan(channel, now); dropped {
			t.Errorf("channel %d should keep the frame on a missed loss roll", channel)
		}
	}
}

func TestPlanDelaysReliableFramesInsteadOfDropping(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, channel := range []int{channelReliableOrdered, channelReliableUnordered} {
		// Loss hits twice, then the retransmit succeeds: 100 + 2 x 200.
		testLink := newTestLink(Profile{LatencyMs: 100, LossPercent: 50}, sequenceRandom(0.0, 0.0, 0.9))
		release, dropped := testLink.plan(channel, now)
		if dropped {
			t.Fatalf("channel %d must never drop", channel)
		}
		if release.Sub(now) != 500*time.Millisecond {
			t.Errorf("channel %d release = %v, want 500ms", channel, release.Sub(now))
		}
	}
}

func TestPlanCapsReliableRetransmits(t *testing.T) {
	now := time.Unix(1000, 0)
	testLink := newTestLink(Profile{LatencyMs: 10, LossPercent: 100}, sequenceRandom(0.0))
	release, dropped := testLink.plan(channelReliableOrdered, now)
	want := time.Duration(10+maxRetransmits*20) * time.Millisecond
	if dropped || release.Sub(now) != want {
		t.Fatalf("release = %v (dropped %v), want %v", release.Sub(now), dropped, want)
	}
}

func TestPlanKeepsOrderedChannelsMonotonic(t *testing.T) {
	now := time.Unix(1000, 0)
	// First frame gets the long delay, the second the short one.
	for _, channel := range []int{channelReliableOrdered, channelUnreliableOrdered} {
		testLink := newTestLink(Profile{LatencyMs: 100, JitterMs: 100}, sequenceRandom(1, 0))
		first, _ := testLink.plan(channel, now)
		second, _ := testLink.plan(channel, now)
		if second.Before(first) {
			t.Errorf("channel %d reordered: %v before %v", channel, second.Sub(now), first.Sub(now))
		}
	}
}

func TestPlanMayReorderUnorderedChannels(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, channel := range []int{channelReliableUnordered, channelUnreliableUnordered} {
		testLink := newTestLink(Profile{LatencyMs: 100, JitterMs: 100}, sequenceRandom(1, 0))
		first, _ := testLink.plan(channel, now)
		second, _ := testLink.plan(channel, now)
		if !second.Before(first) {
			t.Errorf("channel %d should be allowed to reorder", channel)
		}
	}
}

func TestPlanOrderedChannelsAreIndependent(t *testing.T) {
	now := time.Unix(1000, 0)
	testLink := newTestLink(Profile{LatencyMs: 100, JitterMs: 100}, sequenceRandom(1, 0))
	slow, _ := testLink.plan(channelReliableOrdered, now)
	fast, _ := testLink.plan(channelUnreliableOrdered, now)
	if !fast.Before(slow) {
		t.Fatal("the monotonic clamp must not leak across channels")
	}
}

type frameRecorder struct {
	mu       sync.Mutex
	received []int
	arrived  chan struct{}
}

func newFrameRecorder() *frameRecorder {
	return &frameRecorder{arrived: make(chan struct{}, 64)}
}

func (recorder *frameRecorder) deliverer(id int) func() {
	return func() {
		recorder.mu.Lock()
		recorder.received = append(recorder.received, id)
		recorder.mu.Unlock()
		recorder.arrived <- struct{}{}
	}
}

func (recorder *frameRecorder) waitFor(t *testing.T, count int) []int {
	t.Helper()
	for range count {
		select {
		case <-recorder.arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for delivery")
		}
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]int(nil), recorder.received...)
}

func TestSubmitDeliversImmediatelyWhenOff(t *testing.T) {
	testLink := newLink(Profile{})
	defer testLink.close()
	delivered := false
	testLink.submit(channelReliableOrdered, func() { delivered = true })
	if !delivered {
		t.Fatal("an off link must deliver synchronously")
	}
}

func TestSubmitDelaysFrames(t *testing.T) {
	testLink := newLink(Profile{LatencyMs: 60})
	defer testLink.close()
	recorder := newFrameRecorder()

	start := time.Now()
	testLink.submit(channelReliableOrdered, recorder.deliverer(1))
	recorder.waitFor(t, 1)
	if elapsed := time.Since(start); elapsed < 55*time.Millisecond {
		t.Fatalf("frame arrived after %v, want at least 60ms", elapsed)
	}
}

func TestSubmitKeepsOrderUnderJitter(t *testing.T) {
	testLink := newLink(Profile{LatencyMs: 20, JitterMs: 20})
	defer testLink.close()
	recorder := newFrameRecorder()

	const frames = 30
	for id := range frames {
		testLink.submit(channelReliableOrdered, recorder.deliverer(id))
	}
	received := recorder.waitFor(t, frames)
	for index, id := range received {
		if id != index {
			t.Fatalf("frame %d arrived at position %d", id, index)
		}
	}
}

func TestProfileChangeOnlyAffectsLaterFrames(t *testing.T) {
	testLink := newLink(Profile{LatencyMs: 80})
	defer testLink.close()
	recorder := newFrameRecorder()

	testLink.submit(channelReliableOrdered, recorder.deliverer(1))
	testLink.setProfile(Profile{})
	testLink.submit(channelReliableOrdered, recorder.deliverer(2))

	// The off profile must not let frame 2 overtake the still queued frame 1.
	received := recorder.waitFor(t, 2)
	if received[0] != 1 || received[1] != 2 {
		t.Fatalf("received %v, want [1 2]", received)
	}
}

func TestCloseDiscardsQueuedFrames(t *testing.T) {
	testLink := newLink(Profile{LatencyMs: 50})
	recorder := newFrameRecorder()
	testLink.submit(channelReliableOrdered, recorder.deliverer(1))
	testLink.close()

	select {
	case <-recorder.arrived:
		t.Fatal("a closed link must not deliver queued frames")
	case <-time.After(150 * time.Millisecond):
	}
	testLink.submit(channelReliableOrdered, recorder.deliverer(2))
	select {
	case <-recorder.arrived:
		t.Fatal("a closed link must not accept frames")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSubmitDropsUnreliableFramesWhenQueueIsFull(t *testing.T) {
	testLink := newLink(Profile{LatencyMs: 5000})
	defer testLink.close()
	for range maxQueuedFrames {
		testLink.submit(channelReliableOrdered, func() {})
	}
	testLink.submit(channelUnreliableUnordered, func() { t.Error("overflow frame must be dropped") })

	testLink.mu.Lock()
	queued := len(testLink.queue)
	testLink.mu.Unlock()
	if queued != maxQueuedFrames {
		t.Fatalf("queued %d frames, want %d", queued, maxQueuedFrames)
	}
}
