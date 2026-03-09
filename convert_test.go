package parquet_test

import (
	"reflect"
	"testing"

	"github.com/vc42/parquet-go"
)

var conversionTests = [...]struct {
	scenario string
	from     interface{}
	to       interface{}
}{
	{
		scenario: "convert between rows which have the same schema",

		from: AddressBook{
			Owner: "Julien Le Dem",
			OwnerPhoneNumbers: []string{
				"555 123 4567",
				"555 666 1337",
			},
			Contacts: []Contact{
				{
					Name:        "Dmitriy Ryaboy",
					PhoneNumber: "555 987 6543",
				},
				{
					Name: "Chris Aniszczyk",
				},
			},
		},

		to: AddressBook{
			Owner: "Julien Le Dem",
			OwnerPhoneNumbers: []string{
				"555 123 4567",
				"555 666 1337",
			},
			Contacts: []Contact{
				{
					Name:        "Dmitriy Ryaboy",
					PhoneNumber: "555 987 6543",
				},
				{
					Name: "Chris Aniszczyk",
				},
			},
		},
	},

	{
		scenario: "missing column",
		from:     struct{ FirstName, LastName string }{FirstName: "Luke", LastName: "Skywalker"},
		to:       struct{ LastName string }{LastName: "Skywalker"},
	},

	{
		scenario: "missing optional column",
		from: struct {
			FirstName *string
			LastName  string
		}{FirstName: newString("Luke"), LastName: "Skywalker"},
		to: struct{ LastName string }{LastName: "Skywalker"},
	},

	{
		scenario: "missing repeated column",
		from: struct {
			ID    uint64
			Names []string
		}{ID: 42, Names: []string{"me", "myself", "I"}},
		to: struct{ ID uint64 }{ID: 42},
	},

	{
		scenario: "extra column",
		from:     struct{ LastName string }{LastName: "Skywalker"},
		to:       struct{ FirstName, LastName string }{LastName: "Skywalker"},
	},

	{
		scenario: "extra optional column",
		from:     struct{ ID uint64 }{ID: 2},
		to: struct {
			ID      uint64
			Details *struct{ FirstName, LastName string }
		}{ID: 2, Details: nil},
	},

	{
		scenario: "extra repeated column",
		from:     struct{ ID uint64 }{ID: 1},
		to: struct {
			ID    uint64
			Names []string
		}{ID: 1, Names: []string{}},
	},

	// These scenarios pin the two widening shapes that previously broke repeated
	// reconstruction:
	// 1. a missing optional sibling for every repeated element
	// 2. a missing earlier optional sibling plus later present siblings, which used
	//    to expose grouped-by-column rows produced by conversion

	{
		scenario: "repeated struct missing optional sibling",
		from: narrowRepeatedRow{
			Contacts: []narrowRepeatedContact{
				{Name: "Luke"},
				{Name: "Leia"},
			},
		},
		to: wideRepeatedRow{
			Contacts: []wideRepeatedContact{
				{Name: "Luke"},
				{Name: "Leia"},
			},
		},
	},

	{
		scenario: "repeated struct missing and present siblings",
		from: func() narrowMixedRow {
			firstAge := int64(7)
			firstScore := 1.25
			secondScore := 2.5
			return narrowMixedRow{
				Contacts: []narrowMixedContact{
					{Name: "Luke", Score: &firstScore, Age: nil},
					{Name: "Leia", Score: &secondScore, Age: &firstAge},
				},
			}
		}(),
		to: func() wideMixedRow {
			firstAge := int64(7)
			firstScore := 1.25
			secondScore := 2.5
			return wideMixedRow{
				Contacts: []wideMixedContact{
					{Name: "Luke", Score: &firstScore},
					{Name: "Leia", Age: &firstAge, Score: &secondScore},
				},
			}
		}(),
	},
}

func TestConvert(t *testing.T) {
	for _, test := range conversionTests {
		t.Run(test.scenario, func(t *testing.T) {
			to := parquet.SchemaOf(test.to)
			from := parquet.SchemaOf(test.from)

			conv, err := parquet.Convert(to, from)
			if err != nil {
				t.Fatal(err)
			}

			row := from.Deconstruct(nil, test.from)
			row, err = conv.Convert(nil, row)
			if err != nil {
				t.Fatal(err)
			}

			value := reflect.New(reflect.TypeOf(test.to))
			if err := to.Reconstruct(value.Interface(), row); err != nil {
				t.Fatal(err)
			}

			value = value.Elem()
			if !reflect.DeepEqual(value.Interface(), test.to) {
				t.Errorf("converted value mismatch:\nwant = %+v\ngot  = %+v", test.to, value.Interface())
			}
		})
	}
}

type narrowRepeatedContact struct {
	Name string `parquet:"name"`
}

type narrowRepeatedRow struct {
	Contacts []narrowRepeatedContact `parquet:"contacts"`
}

type wideRepeatedContact struct {
	Name        string `parquet:"name"`
	PhoneNumber string `parquet:"phoneNumber,optional"`
}

type wideRepeatedRow struct {
	Contacts []wideRepeatedContact `parquet:"contacts"`
}

type narrowMixedContact struct {
	Name  string   `parquet:"name"`
	Score *float64 `parquet:"score,optional"`
	Age   *int64   `parquet:"age,optional"`
}

type narrowMixedRow struct {
	Contacts []narrowMixedContact `parquet:"contacts"`
}

type wideMixedContact struct {
	Name        string   `parquet:"name"`
	PhoneNumber string   `parquet:"phoneNumber,optional"`
	Age         *int64   `parquet:"age,optional"`
	Score       *float64 `parquet:"score,optional"`
}

type wideMixedRow struct {
	Contacts []wideMixedContact `parquet:"contacts"`
}

func newString(s string) *string { return &s }
