package commands

import "github.com/AlecAivazis/survey/v2"

// Surveyor abstracts interactive prompts for testing.
type Surveyor interface {
	Select(message string, options []string) (string, error)
	MultiSelect(message string, options []string) ([]string, error)
	Input(message string) (string, error)
	InputWithDefault(message, defaultVal string) (string, error)
	Password(message string) (string, error)
}

// defaultSurveyor uses AlecAivazis/survey for real interactive prompts.
type defaultSurveyor struct{}

func (d defaultSurveyor) Select(message string, options []string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &answer)
	return answer, err
}

func (d defaultSurveyor) MultiSelect(message string, options []string) ([]string, error) {
	var answers []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: message,
		Options: options,
	}, &answers)
	return answers, err
}

func (d defaultSurveyor) Input(message string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Input{Message: message}, &answer)
	return answer, err
}

func (d defaultSurveyor) InputWithDefault(message, defaultVal string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Input{Message: message, Default: defaultVal}, &answer)
	return answer, err
}

func (d defaultSurveyor) Password(message string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Password{Message: message}, &answer)
	return answer, err
}

// promptFallbackIssue asks the user to pick a Jira issue key from fallbacks or type one.
func promptFallbackIssue(s Surveyor, fallbacks []string) (string, error) {
	options := make([]string, len(fallbacks))
	copy(options, fallbacks)
	options = append(options, "other (type manually)")

	if len(fallbacks) == 0 {
		// No fallbacks configured — just ask for manual input
		return s.Input("Jira issue key:")
	}

	choice, err := s.Select("Jira issue:", options)
	if err != nil {
		return "", err
	}

	if choice == "other (type manually)" {
		return s.Input("Jira issue key:")
	}
	return choice, nil
}
