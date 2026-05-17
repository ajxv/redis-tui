package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajxv/redis-tui/internal/tui"
)

// ============================================================
// 1. INPUT FLOW  (InputCompleteMsg)
// ============================================================

// TestInput_Key_DispatchingOps verifies that each operation dispatches to the
// correct next state when the user submits the key input form.
func TestInput_Key_DispatchingOps(t *testing.T) {
	cases := []struct {
		op        tui.Op
		wantState tui.AppState
		wantCmd   bool // true = expect a tea.Cmd (Redis dispatch or state transition)
	}{
		{tui.OpGet, tui.StateLoading, true},     // GET dispatches immediately
		{tui.OpSet, tui.StateInputValue, false}, // asks for value next
		{tui.OpRPush, tui.StateInputValue, false},
		{tui.OpSAdd, tui.StateInputValue, false},
		{tui.OpHSet, tui.StateInputField, false}, // asks for field next
		{tui.OpZAdd, tui.StateInputField, false},
		{tui.OpHGet, tui.StateLoading, true},   // HKEYS dispatches
		{tui.OpDelete, tui.StateLoading, true}, // DEL dispatches
		{tui.OpExport, tui.StateInputFilePath, false},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.op
			m.CurrentState = tui.StateInputKey

			m2, cmd := send(m, tui.InputCompleteMsg{Type: tui.InputKey, Value: "mykey"})

			if m2.CurrentState != tc.wantState {
				t.Errorf("state: want %v, got %v", tc.wantState, m2.CurrentState)
			}
			if m2.ActiveKey != "mykey" {
				t.Errorf("ActiveKey: want %q, got %q", "mykey", m2.ActiveKey)
			}
			if tc.wantCmd && cmd == nil {
				t.Error("expected a non-nil tea.Cmd")
			}
		})
	}
}

// TestInput_Pattern_StartsExplore verifies that submitting a SCAN pattern
// transitions to loading and sets the browser cursor to "0".
func TestInput_Pattern_StartsExplore(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExplore
	m.CurrentState = tui.StateInputKey

	m2, cmd := send(m, tui.InputCompleteMsg{Type: tui.InputPattern, Value: "user:*"})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if m2.Browser.Cursor != "0" {
		t.Errorf("cursor: want %q, got %q", "0", m2.Browser.Cursor)
	}
	if m2.Browser.Pattern != "user:*" {
		t.Errorf("pattern: want %q, got %q", "user:*", m2.Browser.Pattern)
	}
	if cmd == nil {
		t.Error("expected a non-nil tea.Cmd")
	}
}

// TestInput_Field_TransitionsToValue verifies that submitting the field form
// moves to value input for hash and sorted-set operations.
func TestInput_Field_TransitionsToValue(t *testing.T) {
	for _, op := range []tui.Op{tui.OpHSet, tui.OpZAdd} {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op
			m.CurrentState = tui.StateInputField

			m2, _ := send(m, tui.InputCompleteMsg{Type: tui.InputField, Value: "score"})

			if m2.CurrentState != tui.StateInputValue {
				t.Errorf("state: want StateInputValue, got %v", m2.CurrentState)
			}
			if m2.ActiveField != "score" {
				t.Errorf("ActiveField: want %q, got %q", "score", m2.ActiveField)
			}
		})
	}
}

// TestInput_Value_DispatchingOps verifies that each operation sends the
// correct Redis command when the value form is submitted.
func TestInput_Value_DispatchingOps(t *testing.T) {
	cases := []struct {
		op    tui.Op
		value string
	}{
		{tui.OpSet, "val"},
		{tui.OpHSet, "val"},
		{tui.OpZAdd, "member"},
		{tui.OpRPush, "val"},
		{tui.OpSAdd, "val"},
		{tui.OpLSet, "newval"},
		{tui.OpRename, "newname"},
		{tui.OpExpirySet, "120"},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.op
			m.ActiveKey = "k"
			m.ActiveField = "f"
			m.CurrentState = tui.StateInputValue

			m2, cmd := send(m, tui.InputCompleteMsg{Type: tui.InputValue, Value: tc.value})

			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if cmd == nil {
				t.Error("expected a non-nil tea.Cmd")
			}
		})
	}
}

// TestInput_ValueZero_Persist verifies that entering "0" for TTL dispatches
// PERSIST (clear expiry) instead of EXPIRE.
func TestInput_ValueZero_Persist(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExpirySet
	m.ActiveKey = "k"
	m.CurrentState = tui.StateInputValue

	m2, cmd := send(m, tui.InputCompleteMsg{Type: tui.InputValue, Value: "0"})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if cmd == nil {
		t.Error("expected a non-nil cmd for PERSIST")
	}
}

// TestInput_Rename_UpdatesActiveKey verifies that a successful rename updates
// ActiveKey to the new name before the command is dispatched.
func TestInput_Rename_UpdatesActiveKey(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpRename
	m.ActiveKey = "old"
	m.CurrentState = tui.StateInputValue

	m2, _ := send(m, tui.InputCompleteMsg{Type: tui.InputValue, Value: "new"})

	if m2.ActiveKey != "new" {
		t.Errorf("ActiveKey: want %q, got %q", "new", m2.ActiveKey)
	}
}

// TestInput_FilePath_DispatchingOps verifies file-path submission routes to
// the right export/import command.
func TestInput_FilePath_DispatchingOps(t *testing.T) {
	ops := []tui.Op{tui.OpExport, tui.OpImport, tui.OpExportDB, tui.OpImportDB}
	for _, op := range ops {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op
			m.ActiveKey = "k"
			m.CurrentState = tui.StateInputFilePath

			m2, cmd := send(m, tui.InputCompleteMsg{Type: tui.InputFilePath, Value: "/tmp/dump.json"})

			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if cmd == nil {
				t.Error("expected a non-nil tea.Cmd")
			}
		})
	}
}

// ============================================================
// 2. REDIS RESULT HANDLING  (RedisResultMsg)
// ============================================================

// TestResult_Get_ShowsOutput verifies GET/HGET results land in StateOutput
// and trigger a TTL fetch (ActiveTTL set to "fetching...").
func TestResult_Get_ShowsOutput(t *testing.T) {
	for _, op := range []tui.Op{tui.OpGet, tui.OpHGet} {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op
			m.ActiveKey = "k"

			m2, cmd := send(m, tui.RedisResultMsg{Result: "hello"})

			if m2.CurrentState != tui.StateOutput {
				t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
			}
			if m2.Output != "hello" {
				t.Errorf("output: want %q, got %q", "hello", m2.Output)
			}
			if m2.ActiveTTL != "fetching..." {
				t.Errorf("ActiveTTL: want %q, got %q", "fetching...", m2.ActiveTTL)
			}
			if cmd == nil {
				t.Error("expected TTL-fetch cmd")
			}
		})
	}
}

// TestResult_Info_NoTTLFetch verifies that INFO results do not trigger a
// TTL fetch.
func TestResult_Info_NoTTLFetch(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpInfo

	m2, cmd := send(m, tui.RedisResultMsg{Result: "# Server\r\nredis_version:7.0"})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
	if m2.ActiveTTL != "" {
		t.Errorf("ActiveTTL: want empty, got %q", m2.ActiveTTL)
	}
	if cmd != nil {
		t.Error("INFO should not trigger a TTL fetch")
	}
}

// TestResult_HKeys_PopulatesBrowserFields verifies that an HKEYS result loads
// the field browser and sets ViewingFields.
func TestResult_HKeys_PopulatesBrowserFields(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpHKeys

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"field1", "field2", "field3"}})

	if m2.CurrentState != tui.StateBrowser {
		t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
	}
	if !m2.Browser.ViewingFields {
		t.Error("ViewingFields: want true")
	}
	if got := len(m2.Browser.FieldsList.Items()); got != 3 {
		t.Errorf("field count: want 3, got %d", got)
	}
}

// TestResult_Explore_SetsHasMore verifies that a non-zero SCAN cursor sets
// HasMore=true on the browser.
func TestResult_Explore_SetsHasMore(t *testing.T) {
	cases := []struct {
		cursor  string
		hasMore bool
	}{
		{"0", false},
		{"42", true},
		{"999", true},
	}
	for _, tc := range cases {
		t.Run("cursor="+tc.cursor, func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tui.OpExplore
			m.Browser.Cursor = "0"

			result := tui.ScanResult{
				Cursor: tc.cursor,
				Keys:   []list.Item{tui.NewListItem("k", "string")},
			}
			m2, _ := send(m, tui.RedisResultMsg{Result: result})

			if m2.CurrentState != tui.StateBrowser {
				t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
			}
			if m2.Browser.HasMore != tc.hasMore {
				t.Errorf("HasMore: want %v, got %v", tc.hasMore, m2.Browser.HasMore)
			}
		})
	}
}

// TestResult_Explore_AppendOnLoadMore verifies that subsequent SCAN pages
// append to the existing key list rather than replacing it.
func TestResult_Explore_AppendOnLoadMore(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExplore
	// Simulate the browser already having one key from a first scan.
	m.Browser.Cursor = "42" // non-zero means "more pages loaded"
	m.Browser.KeyList.SetItems([]list.Item{tui.NewListItem("existing", "string")})

	result := tui.ScanResult{
		Cursor: "0",
		Keys:   []list.Item{tui.NewListItem("new1", "string"), tui.NewListItem("new2", "string")},
	}
	m2, _ := send(m, tui.RedisResultMsg{Result: result})

	if got := len(m2.Browser.KeyList.Items()); got != 3 {
		t.Errorf("key list length: want 3 (1 existing + 2 new), got %d", got)
	}
}

// TestResult_Explore_ReplaceOnFirstPage verifies that a fresh scan (cursor
// starts at "0") replaces the key list rather than appending.
func TestResult_Explore_ReplaceOnFirstPage(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExplore
	m.Browser.Cursor = "0"
	m.Browser.KeyList.SetItems([]list.Item{tui.NewListItem("stale", "string")})

	result := tui.ScanResult{
		Cursor: "0",
		Keys:   []list.Item{tui.NewListItem("fresh", "string")},
	}
	m2, _ := send(m, tui.RedisResultMsg{Result: result})

	items := m2.Browser.KeyList.Items()
	if len(items) != 1 {
		t.Fatalf("key list length: want 1, got %d", len(items))
	}
	if got := items[0].(tui.ListItem).Title(); got != "fresh" {
		t.Errorf("item title: want %q, got %q", "fresh", got)
	}
}

// TestResult_LRange_LoadsListItems verifies an LRANGE result populates the
// field browser with indexed list items and sets OpExploreList.
func TestResult_LRange_LoadsListItems(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpLRange

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"alpha", "beta", "gamma"}})

	if m2.CurrentState != tui.StateBrowser {
		t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpExploreList {
		t.Errorf("SelectedOp: want OpExploreList, got %v", m2.SelectedOp)
	}
	if got := len(m2.Browser.FieldsList.Items()); got != 3 {
		t.Errorf("item count: want 3, got %d", got)
	}
}

// TestResult_SMembers_LoadsSetItems verifies an SSCAN result sets OpExploreSet.
// The response format is [cursor, [member1, member2, ...]] (SSCAN, not SMEMBERS).
func TestResult_SMembers_LoadsSetItems(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpSMembers

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"0", []any{"a", "b"}}})

	if m2.CurrentState != tui.StateBrowser {
		t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpExploreSet {
		t.Errorf("SelectedOp: want OpExploreSet, got %v", m2.SelectedOp)
	}
	if got := len(m2.Browser.FieldsList.Items()); got != 2 {
		t.Errorf("item count: want 2, got %d", got)
	}
}

// TestResult_ZRange_LoadsZSetItemsWithScores verifies that ZRANGE WITHSCORES
// results alternate member/score and set OpExploreZSet.
func TestResult_ZRange_LoadsZSetItemsWithScores(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpZRange

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"member1", "1.5", "member2", "2.0"}})

	if m2.CurrentState != tui.StateBrowser {
		t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpExploreZSet {
		t.Errorf("SelectedOp: want OpExploreZSet, got %v", m2.SelectedOp)
	}
	items := m2.Browser.FieldsList.Items()
	if len(items) != 2 {
		t.Fatalf("item count: want 2, got %d", len(items))
	}
	if desc := items[0].(tui.ListItem).Description(); desc != "Score: 1.5" {
		t.Errorf("description: want %q, got %q", "Score: 1.5", desc)
	}
}

// TestResult_CheckType_Routing verifies that OpCheckType routes each Redis
// type to the correct follow-up command.
func TestResult_CheckType_Routing(t *testing.T) {
	cases := []struct {
		keyType   string
		wantOp    tui.Op
		wantState tui.AppState
	}{
		{"string", tui.OpGet, tui.StateLoading},
		{"hash", tui.OpHKeys, tui.StateLoading},
		{"list", tui.OpLRange, tui.StateLoading},
		{"set", tui.OpSMembers, tui.StateLoading},
		{"zset", tui.OpZRange, tui.StateLoading},
		{"none", tui.OpNone, tui.StateOutput}, // key not found
	}
	for _, tc := range cases {
		t.Run(tc.keyType, func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tui.OpCheckType
			m.ActiveKey = "k"

			m2, cmd := send(m, tui.RedisResultMsg{Result: tc.keyType})

			if m2.CurrentState != tc.wantState {
				t.Errorf("state: want %v, got %v", tc.wantState, m2.CurrentState)
			}
			if tc.wantState == tui.StateLoading {
				if m2.SelectedOp != tc.wantOp {
					t.Errorf("SelectedOp: want %v, got %v", tc.wantOp, m2.SelectedOp)
				}
				if cmd == nil {
					t.Error("expected a non-nil tea.Cmd")
				}
			}
		})
	}
}

// TestResult_DeleteOps_TriggersRefresh verifies that each delete-result op
// dispatches a refresh command for the appropriate list view.
func TestResult_DeleteOps_TriggersRefresh(t *testing.T) {
	cases := []struct {
		op     tui.Op
		wantOp tui.Op
	}{
		{tui.OpDel, tui.OpExplore},
		{tui.OpHDel, tui.OpHKeys},
		{tui.OpLRem, tui.OpLRange},
		{tui.OpSRem, tui.OpSMembers},
		{tui.OpZRem, tui.OpZRange},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.op
			m.ActiveKey = "k"
			m.ActiveField = "f"

			m2, cmd := send(m, tui.RedisResultMsg{Result: 1})

			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if m2.SelectedOp != tc.wantOp {
				t.Errorf("SelectedOp: want %v, got %v", tc.wantOp, m2.SelectedOp)
			}
			if cmd == nil {
				t.Error("expected refresh cmd")
			}
		})
	}
}

// TestResult_TTLFetchedAfterMutation verifies that rename and expiry-set
// results trigger a follow-up TTL fetch.
func TestResult_TTLFetchedAfterMutation(t *testing.T) {
	for _, op := range []tui.Op{tui.OpRename, tui.OpExpirySet} {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op
			m.ActiveKey = "k"

			m2, cmd := send(m, tui.RedisResultMsg{Result: "OK"})

			if m2.CurrentState != tui.StateOutput {
				t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
			}
			if m2.ActiveTTL != "fetching..." {
				t.Errorf("ActiveTTL: want %q, got %q", "fetching...", m2.ActiveTTL)
			}
			if cmd == nil {
				t.Error("expected TTL-fetch cmd")
			}
		})
	}
}

// TestResult_ExpireAfterSet_ResetsOp verifies that OpExpireAfterSet resets
// SelectedOp back to OpSet and fetches the updated TTL.
func TestResult_ExpireAfterSet_ResetsOp(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExpireAfterSet
	m.ActiveKey = "k"
	m.Output = "OK"

	m2, cmd := send(m, tui.RedisResultMsg{Result: 1})

	if m2.SelectedOp != tui.OpSet {
		t.Errorf("SelectedOp: want OpSet, got %v", m2.SelectedOp)
	}
	if m2.ActiveTTL != "fetching..." {
		t.Errorf("ActiveTTL: want %q, got %q", "fetching...", m2.ActiveTTL)
	}
	if cmd == nil {
		t.Error("expected TTL-fetch cmd")
	}
}

// TestResult_IntegerOutput verifies that ops returning an integer count show
// it as the output string.
func TestResult_IntegerOutput(t *testing.T) {
	for _, op := range []tui.Op{tui.OpHSet, tui.OpRPush, tui.OpSAdd, tui.OpZAdd, tui.OpDelete} {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op

			m2, _ := send(m, tui.RedisResultMsg{Result: 3})

			if m2.CurrentState != tui.StateOutput {
				t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
			}
			if m2.Output != "3" {
				t.Errorf("output: want %q, got %q", "3", m2.Output)
			}
		})
	}
}

// TestResult_ApplicationError_ShowsMessage verifies that a non-network error
// (e.g. a Redis ERR reply) transitions to StateOutput with the error text.
func TestResult_ApplicationError_ShowsMessage(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpGet
	m.CurrentState = tui.StateLoading

	m2, _ := send(m, tui.RedisResultMsg{Error: &errString{"WRONGTYPE Operation against a key holding the wrong kind of value"}})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
	if !strings.Contains(m2.Output, "WRONGTYPE") {
		t.Errorf("output should contain error text, got %q", m2.Output)
	}
}

// errString is a plain error (not a net.Error) for testing application-level errors.
type errString struct{ msg string }

func (e *errString) Error() string { return e.msg }

// TestResult_NetworkError_TriggersReconnect verifies that a net.Error causes
// the model to enter StateLoading and return a reconnect command.
func TestResult_NetworkError_TriggersReconnect(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpGet
	m.CurrentState = tui.StateLoading

	m2, cmd := send(m, tui.RedisResultMsg{Error: &mockNetError{}})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if cmd == nil {
		t.Error("expected reconnect cmd")
	}
}

// ============================================================
// 3. KEY / FIELD SELECTION
// ============================================================

// TestSelectKey_DispatchesTypeCheck verifies that selecting a key from the
// browser immediately dispatches a TYPE command.
func TestSelectKey_DispatchesTypeCheck(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateBrowser

	m2, cmd := send(m, tui.SelectKeyMsg{Key: "mykey"})

	if m2.ActiveKey != "mykey" {
		t.Errorf("ActiveKey: want %q, got %q", "mykey", m2.ActiveKey)
	}
	if m2.SelectedOp != tui.OpCheckType {
		t.Errorf("SelectedOp: want OpCheckType, got %v", m2.SelectedOp)
	}
	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if cmd == nil {
		t.Error("expected a non-nil tea.Cmd")
	}
}

// TestSelectField_Hash_DispatchesHGet verifies that selecting a hash field
// dispatches HGET.
func TestSelectField_Hash_DispatchesHGet(t *testing.T) {
	for _, op := range []tui.Op{tui.OpHGet, tui.OpHKeys, tui.OpExplore} {
		t.Run(op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = op
			m.ActiveKey = "myhash"
			m.CurrentState = tui.StateBrowser

			m2, cmd := send(m, tui.SelectFieldMsg{Key: "myhash", Field: "f1"})

			if m2.SelectedOp != tui.OpHGet {
				t.Errorf("SelectedOp: want OpHGet, got %v", m2.SelectedOp)
			}
			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if cmd == nil {
				t.Error("expected HGET cmd")
			}
		})
	}
}

// TestSelectField_List_ShowsDirectOutput verifies that selecting a list item
// displays the item's value directly in StateOutput without a Redis round-trip.
func TestSelectField_List_ShowsDirectOutput(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpExploreList
	m.CurrentState = tui.StateBrowser

	m2, cmd := send(m, tui.SelectFieldMsg{Key: "mylist", Field: "item-value", Index: 2})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
	if m2.Output != "item-value" {
		t.Errorf("output: want %q, got %q", "item-value", m2.Output)
	}
	if cmd != nil {
		t.Error("list item display should not dispatch a Redis command")
	}
}

// ============================================================
// 4. DELETE / RENAME REQUESTS
// ============================================================

// TestDeleteRequest_OpMapping verifies that the correct Op is chosen based on
// context (key vs field, and the current explore mode).
func TestDeleteRequest_OpMapping(t *testing.T) {
	cases := []struct {
		name      string
		currentOp tui.Op
		field     string // empty = key delete
		wantOp    tui.Op
	}{
		{"key", tui.OpExplore, "", tui.OpDel},
		{"hash field", tui.OpHKeys, "f", tui.OpHDel},
		{"list element", tui.OpExploreList, "v", tui.OpLRem},
		{"set member", tui.OpExploreSet, "m", tui.OpSRem},
		{"zset member", tui.OpExploreZSet, "m", tui.OpZRem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.currentOp
			m.CurrentState = tui.StateBrowser

			m2, _ := send(m, tui.DeleteRequestMsg{Key: "k", Field: tc.field})

			if m2.SelectedOp != tc.wantOp {
				t.Errorf("SelectedOp: want %v, got %v", tc.wantOp, m2.SelectedOp)
			}
			if m2.CurrentState != tui.StateConfirmation {
				t.Errorf("state: want StateConfirmation, got %v", m2.CurrentState)
			}
		})
	}
}

// TestRenameRequest_SetsInputState verifies that a rename request pre-fills
// the input with the current key name and transitions to value input.
func TestRenameRequest_SetsInputState(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateBrowser

	m2, _ := send(m, tui.RenameRequestMsg{Key: "oldname"})

	if m2.CurrentState != tui.StateInputValue {
		t.Errorf("state: want StateInputValue, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpRename {
		t.Errorf("SelectedOp: want OpRename, got %v", m2.SelectedOp)
	}
	if m2.Input.Input.Value() != "oldname" {
		t.Errorf("input value: want %q, got %q", "oldname", m2.Input.Input.Value())
	}
}

// ============================================================
// 5. CONFIRMATION DIALOG
// ============================================================

// TestConfirmation_Yes_DispatchesCommand verifies that pressing 'y' sends the
// appropriate Redis command for each confirmable operation.
func TestConfirmation_Yes_DispatchesCommand(t *testing.T) {
	cases := []struct {
		op tui.Op
	}{
		{tui.OpDel},
		{tui.OpHDel},
		{tui.OpLRem},
		{tui.OpSRem},
		{tui.OpZRem},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.op
			m.ActiveKey = "k"
			m.ActiveField = "f"
			m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
			m.CurrentState = tui.StateConfirmation

			m2, cmd := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if cmd == nil {
				t.Error("expected a delete cmd")
			}
		})
	}
}

// TestConfirmation_Yes_Quit verifies that confirming quit returns tea.Quit.
func TestConfirmation_Yes_Quit(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpQuit
	m.StateNavigationHistory = []tui.AppState{tui.StateMenu}
	m.CurrentState = tui.StateConfirmation

	_, cmd := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	// Execute the cmd and verify it produces tea.Quit.
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("cmd() should produce a QuitMsg")
	}
}

// TestConfirmation_Cancel_PopsState verifies that 'n', 'N', and Esc all
// dismiss the confirmation and restore the prior state.
func TestConfirmation_Cancel_PopsState(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'n'}},
		{Type: tea.KeyRunes, Runes: []rune{'N'}},
		{Type: tea.KeyEscape},
	}
	for _, key := range keys {
		t.Run(key.String(), func(t *testing.T) {
			m := newTestModel()
			m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
			m.CurrentState = tui.StateConfirmation
			m.SelectedOp = tui.OpDel

			m2, _ := send(m, key)

			if m2.CurrentState != tui.StateBrowser {
				t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
			}
		})
	}
}

// ============================================================
// 6. STATEOUTPUT KEYBOARD
// ============================================================

// TestOutputKey_Edit_OpRouting verifies that 'e' sets the correct follow-up
// operation for each output context.
func TestOutputKey_Edit_OpRouting(t *testing.T) {
	cases := []struct {
		currentOp tui.Op
		wantOp    tui.Op
	}{
		{tui.OpGet, tui.OpSet},
		{tui.OpHGet, tui.OpHSet},
		{tui.OpExploreList, tui.OpLSet},
	}
	for _, tc := range cases {
		t.Run(tc.currentOp.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.currentOp
			m.Output = "current-value"
			m.ActiveTTL = "no expiry"
			m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
			m.CurrentState = tui.StateOutput

			m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

			if m2.CurrentState != tui.StateInputValue {
				t.Errorf("state: want StateInputValue, got %v", m2.CurrentState)
			}
			if m2.SelectedOp != tc.wantOp {
				t.Errorf("SelectedOp: want %v, got %v", tc.wantOp, m2.SelectedOp)
			}
			if m2.Input.Input.Value() != "current-value" {
				t.Errorf("input pre-filled: want %q, got %q", "current-value", m2.Input.Input.Value())
			}
		})
	}
}

// TestOutputKey_Edit_InfoIsNoop verifies that 'e' does nothing on INFO output.
func TestOutputKey_Edit_InfoIsNoop(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpInfo
	m.Output = "# Server"
	m.CurrentState = tui.StateOutput

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
}

// TestOutputKey_TTL_TransitionsToInput verifies that 'x' enters TTL input mode.
func TestOutputKey_TTL_TransitionsToInput(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpGet
	m.CurrentState = tui.StateOutput
	m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if m2.CurrentState != tui.StateInputValue {
		t.Errorf("state: want StateInputValue, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpExpirySet {
		t.Errorf("SelectedOp: want OpExpirySet, got %v", m2.SelectedOp)
	}
}

// TestOutputKey_TTL_InfoIsNoop verifies that 'x' does nothing on INFO output.
func TestOutputKey_TTL_InfoIsNoop(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpInfo
	m.CurrentState = tui.StateOutput

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
}

// TestOutputKey_Esc_NavigatesBack verifies esc returns to the prior state.
func TestOutputKey_Esc_NavigatesBack(t *testing.T) {
	m := newTestModel()
	m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
	m.CurrentState = tui.StateOutput
	m.Output = "val"
	m.ActiveTTL = "10s"
	m.CopyStatus = "Copied!"

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyEscape})

	if m2.CurrentState != tui.StateBrowser {
		t.Errorf("state: want StateBrowser, got %v", m2.CurrentState)
	}
	if m2.Output != "" {
		t.Errorf("Output should be cleared, got %q", m2.Output)
	}
	if m2.ActiveTTL != "" {
		t.Errorf("ActiveTTL should be cleared, got %q", m2.ActiveTTL)
	}
	if m2.CopyStatus != "" {
		t.Errorf("CopyStatus should be cleared, got %q", m2.CopyStatus)
	}
}

// TestOutputKey_Esc_HardResetOnCreationFlow verifies that escaping when the
// prior state was an input form resets fully to StateMenu.
func TestOutputKey_Esc_HardResetOnCreationFlow(t *testing.T) {
	priors := []struct {
		name  string
		state tui.AppState
	}{
		{"InputKey", tui.StateInputKey},
		{"InputField", tui.StateInputField},
		{"InputFilePath", tui.StateInputFilePath},
	}
	for _, tc := range priors {
		t.Run(tc.name, func(t *testing.T) {
			prior := tc.state
			m := newTestModel()
			m.StateNavigationHistory = []tui.AppState{prior}
			m.CurrentState = tui.StateOutput

			m2, _ := send(m, tea.KeyMsg{Type: tea.KeyEscape})

			if m2.CurrentState != tui.StateMenu {
				t.Errorf("state: want StateMenu, got %v", m2.CurrentState)
			}
			if len(m2.StateNavigationHistory) != 0 {
				t.Errorf("history should be empty after hard reset, got %v", m2.StateNavigationHistory)
			}
		})
	}
}

// TestOutputKey_OpLSet_EscResetsToExploreList verifies that escaping from an
// LSET edit restores SelectedOp to OpExploreList so 'enter' works again.
func TestOutputKey_OpLSet_EscResetsToExploreList(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpLSet
	m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
	m.CurrentState = tui.StateOutput

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyEscape})

	if m2.SelectedOp != tui.OpExploreList {
		t.Errorf("SelectedOp: want OpExploreList, got %v", m2.SelectedOp)
	}
}

// ============================================================
// 7. TTL PRESERVATION
// ============================================================

// TestTTL_PreservedWhenEditing verifies that an active TTL is captured into
// PreservedTTL when the user presses 'e' to edit a value.
func TestTTL_PreservedWhenEditing(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpGet
	m.Output = "val"
	m.ActiveTTL = "120s"
	m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
	m.CurrentState = tui.StateOutput

	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if m2.PreservedTTL != 120 {
		t.Errorf("PreservedTTL: want 120, got %d", m2.PreservedTTL)
	}
}

// TestTTL_NotPreservedWhenNoExpiry verifies that "no expiry" keys set
// PreservedTTL to 0.
func TestTTL_NotPreservedWhenNoExpiry(t *testing.T) {
	for _, activeTTL := range []string{"no expiry", "", "fetching..."} {
		t.Run(activeTTL, func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tui.OpGet
			m.Output = "val"
			m.ActiveTTL = activeTTL
			m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
			m.CurrentState = tui.StateOutput

			m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

			if m2.PreservedTTL != 0 {
				t.Errorf("PreservedTTL: want 0, got %d", m2.PreservedTTL)
			}
		})
	}
}

// TestTTL_ChainedExpireAfterSet verifies that when a SET completes and
// PreservedTTL is set, the model immediately chains an EXPIRE command.
func TestTTL_ChainedExpireAfterSet(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpSet
	m.PreservedTTL = 60
	m.ActiveKey = "k"

	m2, cmd := send(m, tui.RedisResultMsg{Result: "OK"})

	if m2.SelectedOp != tui.OpExpireAfterSet {
		t.Errorf("SelectedOp: want OpExpireAfterSet, got %v", m2.SelectedOp)
	}
	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if m2.PreservedTTL != 0 {
		t.Errorf("PreservedTTL should be cleared after dispatch, got %d", m2.PreservedTTL)
	}
	if cmd == nil {
		t.Error("expected EXPIRE cmd")
	}
}

// TestTTL_NoChainWhenZero verifies that a SET result with no preserved TTL
// does NOT dispatch EXPIRE.
func TestTTL_NoChainWhenZero(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpSet
	m.PreservedTTL = 0
	m.ActiveKey = "k"

	m2, cmd := send(m, tui.RedisResultMsg{Result: "OK"})

	if m2.CurrentState != tui.StateOutput {
		t.Errorf("state: want StateOutput, got %v", m2.CurrentState)
	}
	if cmd != nil {
		t.Error("no EXPIRE should be dispatched when PreservedTTL is 0")
	}
}

// ============================================================
// 8. SYSTEM MESSAGES
// ============================================================

// TestBackMsg_PopsState verifies that a BackMsg returns to the previous state.
func TestBackMsg_PopsState(t *testing.T) {
	m := newTestModel()
	m.StateNavigationHistory = []tui.AppState{tui.StateMenu}
	m.CurrentState = tui.StateBrowser

	m2, _ := send(m, tui.BackMsg{})

	if m2.CurrentState != tui.StateMenu {
		t.Errorf("state: want StateMenu, got %v", m2.CurrentState)
	}
}

// TestConnectionMsg_Success_SetsConn verifies that a successful connection
// message wires the Conn and Reader onto the model.
func TestConnectionMsg_Success_SetsConn(t *testing.T) {
	conn, _ := newMockConn("+OK\r\n")
	m := newTestModel()
	m.CurrentState = tui.StateLoading
	m.StateNavigationHistory = []tui.AppState{tui.StateMenu}

	m2, _ := send(m, tui.RedisConnectionMsg{Conn: conn})

	if m2.Conn == nil {
		t.Error("Conn should be set after successful connection")
	}
	if m2.Reader == nil {
		t.Error("Reader should be set after successful connection")
	}
	// StateLoading should be exited — pop to StateMenu.
	if m2.CurrentState != tui.StateMenu {
		t.Errorf("state: want StateMenu, got %v", m2.CurrentState)
	}
}

// TestConnectionMsg_Error_SchedulesRetry verifies that a failed connection
// schedules a retry (non-nil cmd, stays in loading).
func TestConnectionMsg_Error_SchedulesRetry(t *testing.T) {
	m := newTestModel()

	m2, cmd := send(m, tui.RedisConnectionMsg{Error: &errString{"connection refused"}})

	_ = m2
	if cmd == nil {
		t.Error("expected retry cmd")
	}
}

// TestTickMsg_TriggersReconnect verifies that a TickMsg initiates a reconnect.
func TestTickMsg_TriggersReconnect(t *testing.T) {
	m := newTestModel()
	m.RedisAddress = "localhost:6379"

	_, cmd := send(m, tui.TickMsg{})

	if cmd == nil {
		t.Error("expected a reconnect cmd from TickMsg")
	}
}

// TestClearCopyStatus_ClearsField verifies that ClearCopyStatusMsg empties
// the CopyStatus field.
func TestClearCopyStatus_ClearsField(t *testing.T) {
	m := newTestModel()
	m.CopyStatus = "Copied to clipboard!"

	m2, _ := send(m, tui.ClearCopyStatusMsg{})

	if m2.CopyStatus != "" {
		t.Errorf("CopyStatus: want empty, got %q", m2.CopyStatus)
	}
}

// TestLoadMoreKeys_DispatchesScan verifies that LoadMoreKeysMsg triggers a
// SCAN command with the current cursor.
func TestLoadMoreKeys_DispatchesScan(t *testing.T) {
	m := newTestModel()
	m.Browser.Cursor = "42"
	m.Browser.Pattern = "user:*"
	m.CurrentState = tui.StateBrowser

	m2, cmd := send(m, tui.LoadMoreKeysMsg{})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if cmd == nil {
		t.Error("expected SCAN cmd")
	}
}

// TestRefreshMsg_KeyView_DispatchesScan verifies that refreshing while on the
// key list re-triggers a SCAN.
func TestRefreshMsg_KeyView_DispatchesScan(t *testing.T) {
	m := newTestModel()
	m.Browser.ViewingFields = false
	m.Browser.Pattern = "session:*"
	m.CurrentState = tui.StateBrowser

	m2, cmd := send(m, tui.RefreshMsg{})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpExplore {
		t.Errorf("SelectedOp: want OpExplore, got %v", m2.SelectedOp)
	}
	if cmd == nil {
		t.Error("expected SCAN cmd")
	}
}

// TestRefreshMsg_FieldView_DispatchesTypeCheck verifies that refreshing while
// viewing fields re-triggers a TYPE check on the active key.
func TestRefreshMsg_FieldView_DispatchesTypeCheck(t *testing.T) {
	m := newTestModel()
	m.Browser.ViewingFields = true
	m.ActiveKey = "myhash"
	m.CurrentState = tui.StateBrowser

	m2, cmd := send(m, tui.RefreshMsg{})

	if m2.CurrentState != tui.StateLoading {
		t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
	}
	if m2.SelectedOp != tui.OpCheckType {
		t.Errorf("SelectedOp: want OpCheckType, got %v", m2.SelectedOp)
	}
	if cmd == nil {
		t.Error("expected TYPE cmd")
	}
}

// TestWindowResize_UpdatesDimensions verifies that WindowSizeMsg propagates to
// all tracked dimensions on the model.
func TestWindowResize_UpdatesDimensions(t *testing.T) {
	m := newTestModel()

	m2, _ := send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m2.WindowWidth != 120 {
		t.Errorf("WindowWidth: want 120, got %d", m2.WindowWidth)
	}
	if m2.WindowHeight != 40 {
		t.Errorf("WindowHeight: want 40, got %d", m2.WindowHeight)
	}
}

// TestTTLResult_Display verifies each TTL value is formatted correctly.
func TestTTLResult_Display(t *testing.T) {
	cases := []struct {
		ttl     int
		display string
	}{
		{-1, "no expiry"},
		{-2, "no expiry"},
		{0, "0s"},
		{1, "1s"},
		{3600, "3600s"},
	}
	for _, tc := range cases {
		t.Run(tc.display, func(t *testing.T) {
			m := newTestModel()
			m2, _ := send(m, tui.RedisTTLResultMsg{TTL: tc.ttl})
			if m2.ActiveTTL != tc.display {
				t.Errorf("ActiveTTL: want %q, got %q", tc.display, m2.ActiveTTL)
			}
		})
	}
}

// ============================================================
// 9. NAVIGATION STACK INTEGRITY
// ============================================================

// TestNavStack_NoDuplicatePush verifies the pushState dedup guard: pushing the
// same state twice only records it once.
func TestNavStack_NoDuplicatePush(t *testing.T) {
	m := newTestModel()
	// Transition into StateOutput so StateBrowser is in history.
	m.StateNavigationHistory = []tui.AppState{tui.StateBrowser}
	m.CurrentState = tui.StateOutput
	m.SelectedOp = tui.OpGet
	m.Output = "v"
	m.ActiveTTL = "no expiry"

	// Press 'e' — internally calls pushState(StateOutput).
	// If dedup works, history stays [StateBrowser, StateOutput], not [StateBrowser, StateOutput, StateOutput].
	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if len(m2.StateNavigationHistory) != 2 {
		t.Errorf("history length: want 2, got %d — %v", len(m2.StateNavigationHistory), m2.StateNavigationHistory)
	}
}

// TestNavStack_EscSequence verifies a full back-navigation sequence cleans
// the history stack correctly at each step.
func TestNavStack_EscSequence(t *testing.T) {
	// Build: Menu → Browser → Output
	m := newTestModel()
	m.StateNavigationHistory = []tui.AppState{tui.StateMenu, tui.StateBrowser}
	m.CurrentState = tui.StateOutput
	m.SelectedOp = tui.OpGet

	// Step 1: esc from Output → Browser
	m2, _ := send(m, tea.KeyMsg{Type: tea.KeyEscape})
	if m2.CurrentState != tui.StateBrowser {
		t.Fatalf("step1: want StateBrowser, got %v", m2.CurrentState)
	}

	// Step 2: esc from Browser (via BackMsg in browser component, but
	// we simulate by sending BackMsg directly as the browser would emit)
	m3, _ := send(m2, tui.BackMsg{})
	if m3.CurrentState != tui.StateMenu {
		t.Fatalf("step2: want StateMenu, got %v", m3.CurrentState)
	}
	if len(m3.StateNavigationHistory) != 0 {
		t.Errorf("history should be empty at menu, got %v", m3.StateNavigationHistory)
	}
}

// ============================================================
// 10. VIEW RENDERING
// ============================================================

// TestView_EachState_NonEmpty verifies that View() returns non-empty output
// for every application state.
func TestView_EachState_NonEmpty(t *testing.T) {
	states := []struct {
		name  string
		setup func() tui.Model
	}{
		{"StateMenu", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateMenu
			return m
		}},
		{"StateInputKey", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateInputKey
			m.Input.Type = tui.InputKey
			return m
		}},
		{"StateInputValue", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateInputValue
			m.Input.Type = tui.InputValue
			return m
		}},
		{"StateInputFilePath", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateInputFilePath
			m.Input.Type = tui.InputFilePath
			return m
		}},
		{"StateOutput", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateOutput
			m.Output = "hello"
			m.SelectedOp = tui.OpGet
			return m
		}},
		{"StateLoading", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateLoading
			return m
		}},
		{"StateConfirmation", func() tui.Model {
			m := newTestModel()
			m.CurrentState = tui.StateConfirmation
			m.SelectedOp = tui.OpDel
			m.ActiveKey = "k"
			m.WindowWidth = 80
			m.WindowHeight = 24
			return m
		}},
	}
	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup()
			view := m.View()
			if strings.TrimSpace(view) == "" {
				t.Errorf("View() returned empty output for %s", tc.name)
			}
		})
	}
}

// TestView_Output_ShowsValue verifies that StateOutput renders the stored
// output value.
func TestView_Output_ShowsValue(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateOutput
	m.Output = "redis-value-42"
	m.SelectedOp = tui.OpGet

	if !strings.Contains(m.View(), "redis-value-42") {
		t.Error("View() should contain the output value")
	}
}

// TestView_Output_ShowsTTL verifies that a non-empty ActiveTTL appears in the
// output view.
func TestView_Output_ShowsTTL(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateOutput
	m.Output = "v"
	m.ActiveTTL = "300s"
	m.SelectedOp = tui.OpGet

	if !strings.Contains(m.View(), "300s") {
		t.Error("View() should contain the TTL")
	}
}

// TestView_Output_InfoHelpText verifies that the INFO output uses a restricted
// help text (no edit / TTL keys).
func TestView_Output_InfoHelpText(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateOutput
	m.Output = "# Server"
	m.SelectedOp = tui.OpInfo

	view := m.View()
	if strings.Contains(view, "e: edit") {
		t.Error("INFO output should not show edit hint")
	}
	if !strings.Contains(view, "c: copy") {
		t.Error("INFO output should show copy hint")
	}
}

// TestView_Output_ShowsCopyStatus verifies that a copy notification appears
// in the output view when CopyStatus is set.
func TestView_Output_ShowsCopyStatus(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateOutput
	m.Output = "v"
	m.CopyStatus = "Copied to clipboard!"
	m.SelectedOp = tui.OpGet

	if !strings.Contains(m.View(), "Copied to clipboard!") {
		t.Error("View() should contain CopyStatus")
	}
}

// TestView_Confirmation_ShowsCorrectPrompt verifies that the confirmation
// dialog includes context-appropriate text for each operation.
func TestView_Confirmation_ShowsCorrectPrompt(t *testing.T) {
	cases := []struct {
		op      tui.Op
		key     string
		field   string
		wantStr string
	}{
		{tui.OpDel, "mykey", "", "mykey"},
		{tui.OpHDel, "h", "myfield", "myfield"},
		{tui.OpLRem, "l", "myval", "myval"},
		{tui.OpQuit, "", "", "exit"},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.CurrentState = tui.StateConfirmation
			m.SelectedOp = tc.op
			m.ActiveKey = tc.key
			m.ActiveField = tc.field
			m.WindowWidth = 80
			m.WindowHeight = 24

			if !strings.Contains(m.View(), tc.wantStr) {
				t.Errorf("confirmation view should contain %q", tc.wantStr)
			}
		})
	}
}

// TestView_Loading_ContainsLoadingText verifies StateLoading renders a loading
// indicator.
func TestView_Loading_ContainsLoadingText(t *testing.T) {
	m := newTestModel()
	m.CurrentState = tui.StateLoading

	if !strings.Contains(m.View(), "Loading") {
		t.Error("StateLoading view should contain 'Loading'")
	}
}

// ============================================================
// 11. FIELD PAGINATION
// ============================================================

// TestResult_LRange_HasMoreWhenFullPage verifies that receiving a full page of
// list items sets HasMoreFields=true and advances FieldOffset.
func TestResult_LRange_HasMoreWhenFullPage(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpLRange
	m.Browser.FieldOffset = 0

	// Build exactly 100 items (== fieldPageSize).
	items := make([]any, 100)
	for i := range items {
		items[i] = "item"
	}

	m2, _ := send(m, tui.RedisResultMsg{Result: items})

	if !m2.Browser.HasMoreFields {
		t.Error("HasMoreFields: want true when page is full")
	}
	if m2.Browser.FieldOffset != 100 {
		t.Errorf("FieldOffset: want 100, got %d", m2.Browser.FieldOffset)
	}
	if m2.SelectedOp != tui.OpExploreList {
		t.Errorf("SelectedOp: want OpExploreList, got %v", m2.SelectedOp)
	}
}

// TestResult_LRange_AppendOnLoadMore verifies that a second LRANGE page
// appends to the existing item list rather than replacing it.
func TestResult_LRange_AppendOnLoadMore(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpLRange
	// Simulate the browser already having 100 items from the first page.
	m.Browser.FieldOffset = 100
	m.Browser.FieldsList.SetItems([]list.Item{tui.NewListItem("existing", "string")})

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"new1", "new2"}})

	if got := len(m2.Browser.FieldsList.Items()); got != 3 {
		t.Errorf("item count: want 3 (1 existing + 2 new), got %d", got)
	}
	if m2.Browser.HasMoreFields {
		t.Error("HasMoreFields: want false for partial page")
	}
}

// TestResult_SMembers_CursorPagination verifies that a non-zero SSCAN cursor
// sets HasMoreFields=true and stores the cursor for the next page.
func TestResult_SMembers_CursorPagination(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpSMembers
	m.Browser.FieldCursor = "0" // fresh scan

	m2, _ := send(m, tui.RedisResultMsg{Result: []any{"42", []any{"a", "b", "c"}}})

	if !m2.Browser.HasMoreFields {
		t.Error("HasMoreFields: want true for non-zero cursor")
	}
	if m2.Browser.FieldCursor != "42" {
		t.Errorf("FieldCursor: want %q, got %q", "42", m2.Browser.FieldCursor)
	}
	if got := len(m2.Browser.FieldsList.Items()); got != 3 {
		t.Errorf("item count: want 3, got %d", got)
	}
}

// TestResult_ZRange_HasMoreWhenFullPage verifies that receiving a full page
// (100 member/score pairs) from ZRANGE WITHSCORES sets HasMoreFields=true.
func TestResult_ZRange_HasMoreWhenFullPage(t *testing.T) {
	m := newTestModel()
	m.SelectedOp = tui.OpZRange
	m.Browser.FieldOffset = 0

	// 100 member/score pairs = 200 elements in the flat response.
	resp := make([]any, 200)
	for i := 0; i < 200; i += 2 {
		resp[i] = "member"
		resp[i+1] = "1.0"
	}

	m2, _ := send(m, tui.RedisResultMsg{Result: resp})

	if !m2.Browser.HasMoreFields {
		t.Error("HasMoreFields: want true when full page of zset members")
	}
	if m2.Browser.FieldOffset != 100 {
		t.Errorf("FieldOffset: want 100, got %d", m2.Browser.FieldOffset)
	}
}

// TestLoadMoreFields_DispatchesByOp verifies that LoadMoreFieldsMsg dispatches
// the correct Redis command for each explore operation type.
func TestLoadMoreFields_DispatchesByOp(t *testing.T) {
	cases := []struct {
		op tui.Op
	}{
		{tui.OpExploreList},
		{tui.OpExploreSet},
		{tui.OpExploreZSet},
	}
	for _, tc := range cases {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := newTestModel()
			m.SelectedOp = tc.op
			m.ActiveKey = "mykey"
			m.Browser.FieldOffset = 100
			m.Browser.FieldCursor = "55"
			m.CurrentState = tui.StateBrowser

			m2, cmd := send(m, tui.LoadMoreFieldsMsg{})

			if m2.CurrentState != tui.StateLoading {
				t.Errorf("state: want StateLoading, got %v", m2.CurrentState)
			}
			if cmd == nil {
				t.Error("expected a non-nil tea.Cmd")
			}
		})
	}
}
