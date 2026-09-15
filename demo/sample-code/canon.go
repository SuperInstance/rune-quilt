// canon.go — Go client for the live canon Worker.
//
// The canon is the long-form corpus. Every file in rune-quilt
// gets canonized: embedded via Cloudflare Workers AI and submitted
// to the live canon worker for admission to the canon.
//
// Usage:
//
//   client := canon.New("https://live-canon.casey-digennaro.workers.dev")
//   h, _ := client.GetHash(ctx)
package canon

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    Base string
}

func New(base string) *Client { return &Client{Base: base} }

type Hash struct {
    StateHash   string `json:"state_hash"`
    PaperCount  int    `json:"paper_count"`
    TestCellHash string `json:"test_cell_hash"`
}

func (c *Client) GetHash(ctx context.Context) (*Hash, error) {
    resp, err := http.Get(c.Base + "/api/canon/hash")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var h Hash
    return &h, json.NewDecoder(resp.Body).Decode(&h)
}

func main() {
    h, err := New("https://live-canon.casey-digennaro.workers.dev").GetHash(nil)
    if err != nil {
        fmt.Println("err:", err)
        return
    }
    fmt.Printf("canon state: %s (%d papers)\n", h.StateHash, h.PaperCount)
}
