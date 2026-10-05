package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"task-processor/internal/aiworkbench"
	workstore "task-processor/internal/integration/persistence/aiworkbench"
)

func TestConversationHistoryPagesStayReadableWithinWireLimit(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	require.NoError(t, workstore.InstallSchema(f.owner))
	store, err := workstore.New(f.owner)
	require.NoError(t, err)
	scope := aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}
	conversation, _, err := store.Create(context.Background(), scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	for sequence := 1; sequence <= 17; sequence++ {
		require.NoError(t, f.owner.Exec(`INSERT INTO ai_workbench.messages
			(id, organization_id, owner_user_id, conversation_id, sequence, author_kind, content, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), scope.OrganizationID, scope.ActorID,
			conversation.ID, sequence, "ASSISTANT", strings.Repeat("A", 16<<10), time.Now().UTC()).Error)
	}
	require.NoError(t, f.owner.Exec(`UPDATE ai_workbench.conversations SET next_sequence = 18 WHERE id = ?`, conversation.ID).Error)

	seen := map[uint64]bool{}
	before := ""
	for pageNumber := 0; pageNumber < 17; pageNumber++ {
		path := "/conversation?limit=50"
		if before != "" {
			path += "&before=" + before
		}
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		c.Params = gin.Params{{Key: "conversation_id", Value: conversation.ID}}
		(&aiWorkbenchApplication{store: store}).getConversation(c, context.Background(), scope)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.LessOrEqual(t, response.Body.Len(), 256<<10)
		var page struct {
			Messages []aiworkbench.Message `json:"messages"`
			Before   string                `json:"before"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		require.NotEmpty(t, page.Messages)
		for _, message := range page.Messages {
			require.False(t, seen[message.Sequence], "message repeated across page cursor")
			require.Len(t, message.Content, 16<<10)
			seen[message.Sequence] = true
		}
		if page.Before == "" {
			break
		}
		cursor, err := strconv.ParseUint(page.Before, 10, 64)
		require.NoError(t, err)
		require.Equal(t, page.Messages[0].Sequence, cursor, "cursor must include every omitted older message")
		before = page.Before
	}
	require.Len(t, seen, 17)
}
