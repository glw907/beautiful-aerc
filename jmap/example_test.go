package jmap_test

import (
	"encoding/json"
	"fmt"

	"github.com/glw907/poplar/jmap"
)

// A request chains two calls: the query names the messages, and the
// get fetches them from the query's own result without a round trip.
func ExampleRequest_Invoke() {
	var req jmap.Request
	queryID := req.Invoke(&jmap.EmailQuery{
		Account: "A13824",
		Filter:  &jmap.EmailFilterCondition{InMailbox: "MA"},
		Sort:    []*jmap.Comparator{{Property: "receivedAt", IsAscending: new(false)}},
		Limit:   2,
	})
	req.Invoke(&jmap.EmailGet{
		Account: "A13824",
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryID,
			Name:     "Email/query",
			Path:     "/ids",
		},
		Properties: []string{"subject"},
	})

	body, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
	// Output:
	// {"using":["urn:ietf:params:jmap:core","urn:ietf:params:jmap:mail"],"methodCalls":[["Email/query",{"accountId":"A13824","filter":{"inMailbox":"MA"},"sort":[{"property":"receivedAt","isAscending":false}],"limit":2},"0"],["Email/get",{"accountId":"A13824","properties":["subject"],"#ids":{"resultOf":"0","name":"Email/query","path":"/ids"}},"1"]]}
}

// Marking a message read patches one keyword. Naming the property
// itself would replace the whole keyword set and drop every other
// flag on the message.
func ExamplePointer() {
	patch := jmap.Patch{jmap.Pointer("keywords", "$seen"): true}

	body, err := json.Marshal(&jmap.EmailSet{
		Account: "A13824",
		Update:  map[jmap.ID]jmap.Patch{"M1": patch},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
	// Output:
	// {"accountId":"A13824","update":{"M1":{"keywords/$seen":true}}}
}

// A pointer into an array never reaches the wire. RFC 8620 section
// 5.3 requires the whole property to be replaced instead.
func ExamplePatch_MarshalJSON_illegalPointer() {
	_, err := json.Marshal(jmap.Patch{
		"keywords":                        map[string]bool{"$flagged": true},
		jmap.Pointer("keywords", "$seen"): nil,
	})
	fmt.Println(err)
	// Output:
	// json: error calling MarshalJSON for type jmap.Patch: patch pointers "keywords" and "keywords/$seen" overlap; one is a prefix of the other
}

// A /set answers per record. One create landed and one was refused,
// and the refusal carries the blob ids the server could not find.
func ExampleSetError() {
	const body = `{
	  "methodResponses": [["Email/set", {
	    "accountId": "A13824",
	    "created": {"k1": {"id": "M1"}},
	    "notCreated": {"k2": {"type": "blobNotFound", "notFound": ["G9"]}}
	  }, "0"]],
	  "sessionState": "75128aab4b1b"
	}`

	var resp jmap.Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		panic(err)
	}
	set, ok := resp.Invocations("0")[0].Args.(*jmap.EmailSetResponse)
	if !ok {
		panic("Email/set decoded to the wrong type")
	}

	for creationID, email := range set.Created {
		fmt.Printf("%s created as %s\n", creationID, email.ID)
	}
	for creationID, refused := range set.NotCreated {
		fmt.Printf("%s refused: %v, missing %v\n", creationID, refused, refused.NotFound)
	}
	// Output:
	// k1 created as M1
	// k2 refused: blobNotFound, missing [G9]
}
