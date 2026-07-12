package eater

type ID int64

type Sex string

const (
	SexMale        Sex = "male"
	SexFemale      Sex = "female"
	SexUnspecified Sex = "unspecified"
)

type Eater struct {
	ID   ID
	Name string
	Sex  Sex
}
