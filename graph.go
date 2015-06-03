package main

import "errors"

/* relationship labels.

	*region* is the reference of a course grained physical location.
	*vpc* is an abstract reference to a group of resources.
	*az* is a medium grained reference to an isolated location (DC/floor/whatever).
	*subnet* is an abstract reference to a fixed range pool of IP addresses.
	*instance* is a single guest VM which is located in an az and associated with a vpc.
	*elb* is a logical group of hosts that provide loadbalancing for one or more instances.
			 An elb is located in an az and associated with a vpc.

  (region) -[hosts]-> (vpc)
  (region) <-[hosted_by]- (vpc)

	(region) -[houses]-> (az)
	(region) <-[housed_by]- (az)

	(vpc) -[allocates_network]-> (subnet)
	(vpc) <-[network_allocated_by]- (subnet)

	(az) -[hosts_network]-> (subnet)
	(az) <-[network_hosted_by]- (subnet)

	(subnet) -[ip_allocated_to_instance]-> (instance)
	(subnet) <-[instance_ip_allocated_from]- (instance)

	(subnet) -[homes]-> (elb)
	(subnet) <-[homed_in]- (elb)

	(elb) -[proxies]-> (instance)
	(elb) <-[proxied_by]- (instance)

	===

	(az) -[provisions_elb]-> (elb)
	(az) <-[elb_provisioned_in]- (elb)

	(az) -[provisions_instance]-> (instance)
	(az) <-[instance_provisioned_in]- (instance)

	acl, route, gw
*/

var NodeNotFound = errors.New("Node not found!")
var NeighboursNotFound = errors.New("Neighbours not found!")

type Identity string
type Type uint
type Relationship string
type NodeRef *Node
type Neighbours []*Edge

const InitialNeighbourCapacity = 4

type Node struct {
	Id       Identity
	Type     Type
	Value    interface{}
	vectorId int
}

// EdgeList contains all the relationships between nodes.
type EdgeList struct {
	EdgeCount int
	Edges     map[Identity]Neighbours
}

func (el *EdgeList) Len() int {
	return el.EdgeCount
}

// AddNeighbour
func (el *EdgeList) AddNeighbour(from NodeRef, rel Relationship, to NodeRef) {
	el.EdgeCount++
	neighbours, ok := el.Edges[from.Id]
	if !ok {
		neighbours = make(Neighbours, 0, InitialNeighbourCapacity)
	}
	neighbours = append(neighbours, &Edge{From: from, Relationship: rel, To: to})
	el.Edges[from.Id] = neighbours
}

// GetNeighbours
func (el *EdgeList) GetNeighbours(id Identity) (n Neighbours, err error) {
	n, ok := el.Edges[id]
	if !ok {
		return nil, NeighboursNotFound
	}

	return n, nil
}

type NodeFilterFunc func(n NodeRef) bool

// ByType
func ByType(t Type) (fn NodeFilterFunc) {
	return func(n NodeRef) bool {
		return (n.Type == t)
	}
}

// NodeList contains all of the nodes by Node.Id
type NodeList map[Identity]NodeRef

// AddNode
func (nl NodeList) AddNode(rawId string, t Type, v interface{}) (n *Node) {
	identity := Identity(rawId)

	n = &Node{
		Id:    identity,
		Type:  t,
		Value: v,
	}

	nl[identity] = n

	return n
}

// GetNode
func (nl NodeList) GetNode(id Identity) (n NodeRef, err error) {
	n, ok := nl[id]
	if !ok {
		return nil, NodeNotFound
	}
	return n, nil
}

// GetNodes
func (nl NodeList) GetNodes(filters ...NodeFilterFunc) (nodes []NodeRef) {
	nodes = make([]NodeRef, 0, 16)

	for _, n := range nl {
		nodeMatches := true
		for _, fn := range filters {
			if !fn(n) {
				nodeMatches = false
				break
			}
		}

		if nodeMatches {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

// Len
func (nl NodeList) Len() int {
	return len(nl)
}

// Edge
type Edge struct {
	From         NodeRef
	Relationship Relationship
	To           NodeRef
}

// Graph
type Graph struct {
	NodeList
	*EdgeList
}

// NewGraph
func NewGraph() (g *Graph) {
	return &Graph{
		NewNodeList(),
		NewEdgeList(),
	}
}

// NewEdgeList
func NewEdgeList() (el *EdgeList) {
	return &EdgeList{
		Edges: make(map[Identity]Neighbours),
	}
}

// NewNodeList
func NewNodeList() (nl NodeList) {
	return make(NodeList)
}
