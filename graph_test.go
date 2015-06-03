package main_test

import "testing"
import . "."

func Test_new_NodeList_should_be_empty(t *testing.T) {
	nodeList := NewNodeList()
	if nodeList.Len() != 0 {
		t.Fatalf("nodeList.Len() == %v, want 0", nodeList.Len())
	}
}

func Test_NodeList_AddNode_should_create_and_add_a_node(t *testing.T) {
	nodeList := NewNodeList()
	nodeList.AddNode("vpc123", Vpc, nil)

	if nodeList.Len() != 1 {
		t.Fatalf("nodeList.Len() == %v, want 1", nodeList.Len())
	}

	n, _ := nodeList.GetNode("vpc123")
	if n.Id != "vpc123" {
		t.Fatalf("node.Id = %v, want %v", n.Id, "vpc123")
	}

	if n.Type != Vpc {
		t.Fatalf("node.Type = %v, want %v", n.Type, "vpc")
	}
}

func Test_NodeList_GetNode_with_absent_id_should_return_a_not_found_error(t *testing.T) {
	nodeList := NewNodeList()
	_, err := nodeList.GetNode("vpc123")
	if err != NodeNotFound {
		t.Fatal("err = nil, want NodeNotFound")
	}
}

func Test_NodeList_GetNodes_ByType_should_return_the_correct_list_of_nodes(t *testing.T) {
	nodeList := NewNodeList()
	nodeList.AddNode("vpc123", Vpc, nil)
	nodeList.AddNode("vpc345", Vpc, nil)
	nodeList.AddNode("i-123abc", Instance, nil)

	nodes := nodeList.GetNodes(ByType(Instance))
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %v, want %v", len(nodes), 1)
	}
}

func Test_NodeList_GetNodes_ByType_should_return_an_empty_list_when_type_not_available(t *testing.T) {
	nodeList := NewNodeList()
	nodeList.AddNode("vpc123", Vpc, nil)
	nodeList.AddNode("vpc345", Vpc, nil)

	nodes := nodeList.GetNodes(ByType(Instance))
	if len(nodes) != 0 {
		t.Fatalf("len(nodes) = %v, want %v", len(nodes), 0)
	}
}

func Test_EdgeList_should_have_empty_Len_after_creation(t *testing.T) {
	el := NewEdgeList()

	if el.Len() != 0 {
		t.Fatalf("el.Len() = %v, want 0", el.Len())
	}
}

func Test_EdgeList_GetNeighbours_should_return_no_edge_found_when_no_edges_are_available(t *testing.T) {
	el := NewEdgeList()
	_, err := el.GetNeighbours("vpc123")
	if err != NeighboursNotFound {
		t.Fatalf("err = %v, want NeighboursNotFound", err)
	}
}

func Test_EdgeList_AddNeighbour_should_make_the_relationship_retrievable(t *testing.T) {
	el := NewEdgeList()
	n1 := &Node{Id: "vpc123"}
	n2 := &Node{Id: "vpc456"}

	el.AddNeighbour(n1, "member of", n2)

	if el.Len() != 1 {
		t.Fatalf("el.Len() = %v, want 1", el.Len())
	}

	neighbours, _ := el.GetNeighbours(n1.Id)
	if neighbours == nil {
		t.Fatal("neighbours = nil, want n1")
	}

	if len(neighbours) != 1 {
		t.Fatalf("len(neighbours) = %v, want 1", len(neighbours))
	}
}
