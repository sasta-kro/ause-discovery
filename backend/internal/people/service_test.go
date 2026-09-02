package people

import "testing"

func TestValidateStudentID(t *testing.T) {
	studentID := "1234567"
	if _, _, _, err := validate(Input{DisplayName: "Student Name", StudentID: &studentID}); err != nil {
		t.Fatalf("expected valid student id: %v", err)
	}
	invalidStudentID := "1234abc"
	if _, _, _, err := validate(Input{DisplayName: "Student Name", StudentID: &invalidStudentID}); err == nil {
		t.Fatal("expected invalid student id rejection")
	}
}
