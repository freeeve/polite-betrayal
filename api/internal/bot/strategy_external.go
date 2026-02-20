package bot

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// ExternalOption configures an ExternalStrategy before launch.
type ExternalOption func(*ExternalStrategy)

// WithMoveTime sets the time budget (in milliseconds) for the engine's go command.
func WithMoveTime(ms int) ExternalOption {
	return func(e *ExternalStrategy) {
		e.moveTimeMs = ms
	}
}

// WithTimeout sets the overall deadline for reading a bestorders response.
// If the engine hasn't responded within this duration after "go", the strategy
// sends "stop" and reads the forced bestorders.
func WithTimeout(d time.Duration) ExternalOption {
	return func(e *ExternalStrategy) {
		e.timeout = d
	}
}

// WithEngineOption queues a "setoption" command to send during handshake.
func WithEngineOption(name, value string) ExternalOption {
	return func(e *ExternalStrategy) {
		e.options = append(e.options, engineOption{name: name, value: value})
	}
}

// engineOption is a name/value pair sent via "setoption name <n> value <v>".
type engineOption struct {
	name  string
	value string
}

// ExternalStrategy implements Strategy by delegating to an external DUI engine process.
type ExternalStrategy struct {
	enginePath string
	power      diplomacy.Power
	moveTimeMs int
	timeout    time.Duration
	options    []engineOption

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner

	mu     sync.Mutex
	closed bool

	// exited is closed when the process exits; used by isAlive.
	exited chan struct{}

	// lastPressOut holds press_out lines from the last engine query.
	lastPressOut []string
}

// NewExternalStrategy spawns the engine process, performs the DUI handshake
// (dui -> duiok, setoptions, isready -> readyok), and returns a ready strategy.
func NewExternalStrategy(enginePath string, power diplomacy.Power, opts ...ExternalOption) (*ExternalStrategy, error) {
	e := &ExternalStrategy{
		enginePath: enginePath,
		power:      power,
		moveTimeMs: 5000,
		timeout:    10 * time.Second,
	}
	for _, o := range opts {
		o(e)
	}

	if err := e.start(); err != nil {
		return nil, fmt.Errorf("external strategy: start engine: %w", err)
	}

	if err := e.handshake(); err != nil {
		e.Close()
		return nil, fmt.Errorf("external strategy: handshake: %w", err)
	}

	return e, nil
}

// Name returns the strategy name.
func (e *ExternalStrategy) Name() string { return "realpolitik" }

// GenerateMovementOrders sends the position to the engine and converts the DSON
// bestorders response into movement-phase OrderInputs.
func (e *ExternalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	dsonOrders, err := e.queryEngine(gs, power)
	if err != nil {
		log.Printf("external strategy: movement orders failed: %v; falling back to hold", err)
		return holdAll(gs, power)
	}

	var inputs []OrderInput
	for _, d := range dsonOrders {
		o := diplomacy.DSONToOrder(d, power)
		inputs = append(inputs, orderToInput(o))
	}
	return inputs
}

// GenerateRetreatOrders sends the position to the engine and converts the DSON
// bestorders response into retreat-phase OrderInputs.
func (e *ExternalStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	dsonOrders, err := e.queryEngine(gs, power)
	if err != nil {
		log.Printf("external strategy: retreat orders failed: %v; falling back to disband", err)
		return disbandAllDislodged(gs, power)
	}

	var inputs []OrderInput
	for _, d := range dsonOrders {
		ro := diplomacy.DSONToRetreatOrder(d, power)
		inputs = append(inputs, retreatOrderToInput(ro))
	}
	return inputs
}

// GenerateBuildOrders sends the position to the engine and converts the DSON
// bestorders response into build-phase OrderInputs.
func (e *ExternalStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	dsonOrders, err := e.queryEngine(gs, power)
	if err != nil {
		log.Printf("external strategy: build orders failed: %v; falling back to waive/civil disorder", err)
		return nil
	}

	var inputs []OrderInput
	for _, d := range dsonOrders {
		bo := diplomacy.DSONToBuildOrder(d, power)
		inputs = append(inputs, buildOrderToInput(bo))
	}
	return inputs
}

// Close sends "quit" to the engine and waits for process exit. If the process
// does not exit within 3 seconds, it is forcefully killed.
func (e *ExternalStrategy) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	// Send quit while stdin is still open and before marking closed.
	if e.stdin != nil {
		fmt.Fprintf(e.stdin, "quit\n")
	}
	e.closed = true
	e.mu.Unlock()

	if e.stdin != nil {
		e.stdin.Close()
	}

	if e.exited != nil {
		select {
		case <-e.exited:
			// Process already exited.
		case <-time.After(3 * time.Second):
			log.Printf("external strategy: engine did not exit within 3s, killing")
			if e.cmd != nil && e.cmd.Process != nil {
				e.cmd.Process.Kill()
			}
			<-e.exited
		}
	}
	return nil
}

// start launches the engine subprocess and starts a goroutine to track exit.
func (e *ExternalStrategy) start() error {
	e.cmd = exec.Command(e.enginePath)

	var err error
	e.stdin, err = e.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	e.scanner = bufio.NewScanner(stdout)
	e.exited = make(chan struct{})

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	// Track process exit in background so isAlive can check without blocking.
	go func() {
		e.cmd.Wait()
		close(e.exited)
	}()

	return nil
}

// handshake performs the DUI initialization sequence.
func (e *ExternalStrategy) handshake() error {
	e.send("dui")
	if err := e.readUntil("duiok"); err != nil {
		return fmt.Errorf("waiting for duiok: %w", err)
	}

	for _, opt := range e.options {
		if opt.value != "" {
			e.send(fmt.Sprintf("setoption name %s value %s", opt.name, opt.value))
		} else {
			e.send(fmt.Sprintf("setoption name %s", opt.name))
		}
	}

	e.send("isready")
	if err := e.readUntil("readyok"); err != nil {
		return fmt.Errorf("waiting for readyok: %w", err)
	}

	return nil
}

// queryEngine sends position + setpower + go to the engine and reads the
// bestorders response. Returns parsed DSONOrders or an error.
func (e *ExternalStrategy) queryEngine(gs *diplomacy.GameState, power diplomacy.Power) ([]diplomacy.DSONOrder, error) {
	return e.queryEngineWithPress(gs, power, nil)
}

// queryEngineWithPress sends press messages, position, setpower, and go to the engine.
// Returns parsed DSONOrders and captures press_out responses.
func (e *ExternalStrategy) queryEngineWithPress(gs *diplomacy.GameState, power diplomacy.Power, pressMessages []DiplomaticIntent) ([]diplomacy.DSONOrder, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("engine is closed")
	}
	e.mu.Unlock()

	if !e.isAlive() {
		return nil, fmt.Errorf("engine process is not running")
	}

	dfen := diplomacy.EncodeDFEN(gs)
	e.send(fmt.Sprintf("position %s", dfen))
	e.send(fmt.Sprintf("setpower %s", string(power)))

	// Send press messages before go
	for _, msg := range pressMessages {
		pressCmd := formatPressDUI(msg)
		if pressCmd != "" {
			e.send(pressCmd)
		}
	}

	e.send(fmt.Sprintf("go movetime %d", e.moveTimeMs))

	resp, err := e.readEngineResponse()
	if err != nil {
		return nil, fmt.Errorf("reading engine response: %w", err)
	}

	// Store press_out for later retrieval
	e.lastPressOut = resp.pressOut

	orderStr := strings.TrimPrefix(resp.bestorders, "bestorders ")
	orders, err := diplomacy.ParseDSON(orderStr)
	if err != nil {
		return nil, fmt.Errorf("parsing DSON response %q: %w", orderStr, err)
	}

	return orders, nil
}

// engineResponse holds the bestorders line and any press_out lines.
type engineResponse struct {
	bestorders string
	pressOut   []string
}

// readEngineResponse reads lines from the engine, collecting any press_out lines
// that appear before bestorders. The engine emits press_out before bestorders so
// the reader doesn't block waiting for more output after bestorders.
// If the timeout is exceeded, it sends "stop" and reads one more time.
func (e *ExternalStrategy) readEngineResponse() (engineResponse, error) {
	type result struct {
		resp engineResponse
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		var resp engineResponse

		for e.scanner.Scan() {
			line := e.scanner.Text()

			// Collect press_out lines emitted before bestorders
			if strings.HasPrefix(line, "press_out ") {
				resp.pressOut = append(resp.pressOut, line)
				continue
			}

			if strings.HasPrefix(line, "bestorders ") {
				resp.bestorders = line
				ch <- result{resp: resp}
				return
			}

			// Skip info lines
		}

		if err := e.scanner.Err(); err != nil {
			ch <- result{err: fmt.Errorf("scanner: %w", err)}
		} else {
			ch <- result{err: fmt.Errorf("engine closed stdout unexpectedly")}
		}
	}()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-time.After(e.timeout):
		e.send("stop")
		// Give engine a short grace period to emit bestorders after stop.
		select {
		case r := <-ch:
			return r.resp, r.err
		case <-time.After(2 * time.Second):
			return engineResponse{}, fmt.Errorf("engine did not respond to stop within 2s")
		}
	}
}

// send writes a command line to the engine's stdin.
func (e *ExternalStrategy) send(line string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.stdin == nil {
		return
	}
	fmt.Fprintf(e.stdin, "%s\n", line)
}

// readUntil reads lines from the engine until the expected line is seen.
// Lines not matching are ignored (id, option, info lines, etc).
func (e *ExternalStrategy) readUntil(expected string) error {
	deadline := time.After(e.timeout)
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		for e.scanner.Scan() {
			line := e.scanner.Text()
			if line == expected {
				ch <- line
				return
			}
		}
		if err := e.scanner.Err(); err != nil {
			errCh <- err
		} else {
			errCh <- fmt.Errorf("engine closed stdout before sending %q", expected)
		}
	}()

	select {
	case <-ch:
		return nil
	case err := <-errCh:
		return err
	case <-deadline:
		return fmt.Errorf("timeout waiting for %q", expected)
	}
}

// isAlive checks whether the engine process is still running.
func (e *ExternalStrategy) isAlive() bool {
	if e.exited == nil {
		return false
	}
	select {
	case <-e.exited:
		return false
	default:
		return true
	}
}

// holdAll generates hold orders for all units belonging to the power.
func holdAll(gs *diplomacy.GameState, power diplomacy.Power) []OrderInput {
	var orders []OrderInput
	for _, u := range gs.UnitsOf(power) {
		orders = append(orders, OrderInput{
			UnitType:  u.Type.String(),
			Location:  u.Province,
			Coast:     string(u.Coast),
			OrderType: "hold",
		})
	}
	return orders
}

// disbandAllDislodged generates disband orders for all dislodged units of the power.
func disbandAllDislodged(gs *diplomacy.GameState, power diplomacy.Power) []OrderInput {
	var orders []OrderInput
	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}
		orders = append(orders, OrderInput{
			UnitType:  d.Unit.Type.String(),
			Location:  d.DislodgedFrom,
			Coast:     string(d.Unit.Coast),
			OrderType: "retreat_disband",
		})
	}
	return orders
}

// orderToInput converts a diplomacy.Order to a bot OrderInput.
func orderToInput(o diplomacy.Order) OrderInput {
	return OrderInput{
		UnitType:    o.UnitType.String(),
		Location:    o.Location,
		Coast:       string(o.Coast),
		OrderType:   orderTypeToString(o.Type),
		Target:      o.Target,
		TargetCoast: string(o.TargetCoast),
		AuxLoc:      o.AuxLoc,
		AuxTarget:   o.AuxTarget,
		AuxUnitType: o.AuxUnitType.String(),
	}
}

// retreatOrderToInput converts a diplomacy.RetreatOrder to a bot OrderInput.
func retreatOrderToInput(ro diplomacy.RetreatOrder) OrderInput {
	orderType := "retreat_move"
	if ro.Type == diplomacy.RetreatDisband {
		orderType = "retreat_disband"
	}
	return OrderInput{
		UnitType:    ro.UnitType.String(),
		Location:    ro.Location,
		Coast:       string(ro.Coast),
		OrderType:   orderType,
		Target:      ro.Target,
		TargetCoast: string(ro.TargetCoast),
	}
}

// buildOrderToInput converts a diplomacy.BuildOrder to a bot OrderInput.
func buildOrderToInput(bo diplomacy.BuildOrder) OrderInput {
	orderType := "build"
	switch bo.Type {
	case diplomacy.DisbandUnit:
		orderType = "disband"
	case diplomacy.WaiveBuild:
		orderType = "waive"
	}
	return OrderInput{
		UnitType:  bo.UnitType.String(),
		Location:  bo.Location,
		Coast:     string(bo.Coast),
		OrderType: orderType,
	}
}

// GenerateDiplomaticMessages implements DiplomaticStrategy for ExternalStrategy.
// Sends received press to the engine and returns outbound press from the engine.
func (e *ExternalStrategy) GenerateDiplomaticMessages(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap, received []DiplomaticIntent) []DiplomaticIntent {
	// The press was already sent during the last queryEngineWithPress call.
	// Parse any press_out lines from the engine into DiplomaticIntents.
	var responses []DiplomaticIntent
	for _, line := range e.lastPressOut {
		if intent := parsePressDUIOut(line, power); intent != nil {
			responses = append(responses, *intent)
		}
	}
	return responses
}

// formatPressDUI converts a DiplomaticIntent into a DUI press command.
// Format: press <from_power> <message_type> [args...]
func formatPressDUI(intent DiplomaticIntent) string {
	from := strings.ToLower(string(intent.From))
	switch intent.Type {
	case IntentRequestSupport:
		if len(intent.Provinces) >= 2 {
			return fmt.Sprintf("press %s request_support %s %s", from, intent.Provinces[0], intent.Provinces[1])
		}
		return ""
	case IntentProposeNonAggression:
		if len(intent.Provinces) > 0 {
			return fmt.Sprintf("press %s propose_nonaggression %s", from, strings.Join(intent.Provinces, " "))
		}
		return fmt.Sprintf("press %s propose_nonaggression", from)
	case IntentProposeAlliance:
		if intent.TargetPower != "" {
			return fmt.Sprintf("press %s propose_alliance against %s", from, strings.ToLower(string(intent.TargetPower)))
		}
		return fmt.Sprintf("press %s propose_alliance", from)
	case IntentThreaten:
		if len(intent.Provinces) > 0 {
			return fmt.Sprintf("press %s threaten %s", from, intent.Provinces[0])
		}
		return ""
	case IntentOfferDeal:
		if len(intent.Provinces) >= 2 {
			return fmt.Sprintf("press %s offer_deal %s %s", from, intent.Provinces[0], intent.Provinces[1])
		}
		return ""
	case IntentAccept:
		return fmt.Sprintf("press %s accept", from)
	case IntentReject:
		return fmt.Sprintf("press %s reject", from)
	}
	return ""
}

// parsePressDUIOut parses a "press_out <to_power> <type> [args...]" line into a DiplomaticIntent.
func parsePressDUIOut(line string, from diplomacy.Power) *DiplomaticIntent {
	raw := strings.TrimPrefix(line, "press_out ")
	tokens := strings.Fields(raw)
	if len(tokens) < 2 {
		return nil
	}

	to := diplomacy.Power(tokens[0])

	switch tokens[1] {
	case "request_support":
		if len(tokens) < 4 {
			return nil
		}
		return &DiplomaticIntent{
			Type:      IntentRequestSupport,
			From:      from,
			To:        to,
			Provinces: []string{tokens[2], tokens[3]},
		}
	case "propose_nonaggression":
		provs := tokens[2:]
		return &DiplomaticIntent{
			Type:      IntentProposeNonAggression,
			From:      from,
			To:        to,
			Provinces: provs,
		}
	case "propose_alliance":
		var target diplomacy.Power
		if len(tokens) >= 4 && tokens[2] == "against" {
			target = diplomacy.Power(tokens[3])
		}
		return &DiplomaticIntent{
			Type:        IntentProposeAlliance,
			From:        from,
			To:          to,
			TargetPower: target,
		}
	case "threaten":
		if len(tokens) < 3 {
			return nil
		}
		return &DiplomaticIntent{
			Type:      IntentThreaten,
			From:      from,
			To:        to,
			Provinces: []string{tokens[2]},
		}
	case "offer_deal":
		if len(tokens) < 4 {
			return nil
		}
		return &DiplomaticIntent{
			Type:      IntentOfferDeal,
			From:      from,
			To:        to,
			Provinces: []string{tokens[2], tokens[3]},
		}
	case "accept":
		return &DiplomaticIntent{
			Type: IntentAccept,
			From: from,
			To:   to,
		}
	case "reject":
		return &DiplomaticIntent{
			Type: IntentReject,
			From: from,
			To:   to,
		}
	}
	return nil
}
