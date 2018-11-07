package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

var (
	metaCards   []*github.ProjectCard
	metaColumns []*github.ProjectColumn
	cardsLock   sync.Mutex
)

func getColumnCards(column *github.ProjectColumn) ([]*github.ProjectCard, error) {
	var ret []*github.ProjectCard

	ctx := context.Background()
	opts := &github.ProjectCardListOptions{}

	for {
		cds, resp, err := client.Projects.ListProjectCards(ctx, column.GetID(), opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, cds...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func getProjectColumns(project *github.Project) ([]*github.ProjectColumn, error) {
	var ret []*github.ProjectColumn

	ctx := context.Background()
	opts := &github.ListOptions{}

	for {
		colns, resp, err := client.Projects.ListProjectColumns(ctx, project.GetID(), opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, colns...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func getProjects() ([]*github.Project, error) {
	var ret []*github.Project

	ctx := context.Background()
	opts := &github.ProjectListOptions{}

	for {
		projs, resp, err := client.Organizations.ListProjects(ctx, OrgName, opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, projs...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func findCard(card *github.ProjectCard) int {
	for i, cd := range metaCards {
		if cd.GetID() == card.GetID() {
			return i
		}
	}
	return -1
}

func AppendCard(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	metaCards = append(metaCards, card)
	return nil
}

func RemoveCard(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	index := findCard(card)
	if index == -1 {
		return errors.New("card's not in project")
	}

	metaCards[index] = metaCards[len(metaCards)-1]
	metaCards[len(metaCards)-1] = nil
	metaCards = metaCards[:len(metaCards)-1]
	return nil
}

func ConvertCard(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	index := findCard(card)
	if index == -1 {
		return errors.New("card's not in project")
	}

	metaCards[index] = card
	return nil
}

func UpdateKanbanMetadata() error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	metaCards = []*github.ProjectCard{}
	metaColumns = []*github.ProjectColumn{}

	projects, err := getProjects()
	if err != nil {
		return err
	}
	for _, pro := range projects {
		if pro.GetName() == TargetProject {
			columns, err := getProjectColumns(pro)
			if err != nil {
				return err
			}
			for _, col := range columns {
				cards, err := getColumnCards(col)
				if err != nil {
					return err
				}
				for _, card := range cards {
					columnID := col.GetID()
					card.ColumnID = &columnID
					metaCards = append(metaCards, card)
				}

				logrus.Infof("got %v cards in column \"%v\"", len(cards), col.GetName())
			}
			logrus.Infof("got total %v cards in project \"%v\"", len(metaCards), TargetProject)

			metaColumns = append(metaColumns, columns...)
		}
	}

	return nil
}

func moveCard(card *github.ProjectCard, column *github.ProjectColumn) error {
	ctx := context.Background()
	opts := &github.ProjectCardMoveOptions{
		Position: "top",
		ColumnID: column.GetID(),
	}

	_, err := client.Projects.MoveProjectCard(ctx, card.GetID(), opts)
	if err != nil {
		return err
	}
	return nil
}

func moveIssue(issue *github.Issue, column *github.ProjectColumn) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	for _, card := range metaCards {
		if card.GetContentURL() == issue.GetURL() && card.GetColumnID() != column.GetID() {
			err := moveCard(card, column)
			if err == nil {
				columnID := column.GetID()
				card.ColumnID = &columnID
			}
			return err
		}
	}
	return errors.New("the issue is not in the project")
}

func moveIssueToColumn(issue *github.Issue, columnName string) error {
	for _, col := range metaColumns {
		if col.GetName() == columnName {
			return moveIssue(issue, col)
		}
	}
	return fmt.Errorf("no column named %v in project %v", columnName, TargetProject)
}

func MoveToTesting(issue *github.Issue) error {
	return moveIssueToColumn(issue, TestingColumnName)
}

func MoveToDeveloping(issue *github.Issue) error {
	return moveIssueToColumn(issue, DevelopingColumnName)
}

func GetIssueColumn(issue *github.Issue) (*github.ProjectColumn, error) {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	for _, card := range metaCards {
		if card.GetContentURL() == issue.GetURL() {
			for _, col := range metaColumns {
				if col.GetID() == card.GetColumnID() {
					return col, nil
				}
			}
		}
	}

	return nil, errors.New("the issue is not in the project")
}
