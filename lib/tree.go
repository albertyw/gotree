/*
   Package gotree implements a simple
   library for handling phylogenetic trees in go
*/
package lib

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/fredericlemoine/bitset"
	"github.com/fredericlemoine/gotree/lib/nodeindex"
	"math"
	"math/rand"
	"sort"
	"strconv"
)

type Tree struct {
	root     *Node           // root node: If the tree is unrooted the root node should have 3 children
	tipIndex map[string]uint // Map between tip name and bitset index
}

type Node struct {
	name    string   // Name of the node
	comment []string // Comment if any in the newick file
	neigh   []*Node  // neighbors array
	br      []*Edge  // Branches array (same order than neigh)
	depth   int      // Depth of the node
}

type Edge struct {
	left, right *Node   // Left and right nodes
	length      float64 // length of branch
	support     float64 // -1 if no support
	// a Bit at index i in the bitset corresponds to the position of the tip i
	//left:0/right:1 .
	// i is the index of the tip in the sorted tip name array
	bitset *bitset.BitSet // Bitset of length Number of taxa each
}

func (t *Tree) NewNode() *Node {
	return &Node{
		name:    "",
		comment: make([]string, 0),
		neigh:   make([]*Node, 0, 3),
		br:      make([]*Edge, 0, 3),
		depth:   0,
	}
}

// put at nil the node and all its branches
func (t *Tree) delNode(n *Node) {
	for i, _ := range n.neigh {
		n.neigh[i] = nil
	}
	n.neigh = nil

	for i, e := range n.br {
		e.left = nil
		e.right = nil
		n.br[i] = nil
	}
	n.br = nil
}

func (t *Tree) NewEdge() *Edge {
	return &Edge{
		length:  -1.0,
		support: -1.0,
	}
}

func NewTree() *Tree {
	return &Tree{
		root:     nil,
		tipIndex: make(map[string]uint, 0),
	}
}

/* Node functions */
/******************/

// Adds a child n to the node p, connected with edge e
func (p *Node) addChild(n *Node, e *Edge) {
	p.neigh = append(p.neigh, n)
	p.br = append(p.br, e)

}

func (n *Node) SetName(name string) {
	n.name = name
}

func (n *Node) AddComment(comment string) {
	n.comment = append(n.comment, comment)
}

func (n *Node) SetDepth(depth int) {
	n.depth = depth
}

func (n *Node) Name() string {
	return n.name
}

func (n *Node) delNeighbor(n2 *Node) error {
	i, err := n.NodeIndex(n2)
	if err != nil {
		return err
	}
	n.br = append(n.br[0:i], n.br[i+1:]...)
	n.neigh = append(n.neigh[0:i], n.neigh[i+1:]...)
	return nil
}

// Retrieve the parent node
// If several parents: Error
// Parent is defined as the node n2 connected to n
// by an edge e with e.left == n2 and e.right == n
func (n *Node) Parent() (*Node, error) {
	var n2 *Node
	for _, e := range n.br {
		if e.right == n {
			if n2 != nil {
				return nil, errors.New("The node has more than one parent")
			}
			n2 = e.left
		}
	}
	if n2 == nil {
		return nil, errors.New("The node has no parent : May be the root?")
	}
	return n2, nil
}

// Retrieve the Edge going to Parent node
// If several parents: Error
// Parent is defined as the node n2 connected to n
// by an edge e with e.left == n2 and e.right == n
func (n *Node) ParentEdge() (*Edge, error) {
	var e2 *Edge
	for _, e := range n.br {
		if e.right == n {
			if e2 != nil {
				return nil, errors.New("The node has more than one parent")
			}
			e2 = e
		}
	}
	if e2 == nil {
		return nil, errors.New("The node has no parent : May be the root?")
	}
	return e2, nil
}

/* Edge functions */
/******************/

func (e *Edge) setLeft(left *Node) {
	e.left = left
}
func (e *Edge) setRight(right *Node) {
	e.right = right
}
func (e *Edge) SetLength(length float64) {
	e.length = length
}

func (e *Edge) SetSupport(support float64) {
	e.support = support
}

func (e *Edge) Length() float64 {
	return e.length
}

func (e *Edge) Support() float64 {
	return e.support
}

func (e *Edge) DumpBitSet() string {
	if e.bitset == nil {
		return "nil"
	}
	return e.bitset.DumpAsBits()
}

/* Tree functions */
/******************/

func (t *Tree) SetRoot(r *Node) {
	t.root = r
}

func (t *Tree) Root() *Node {
	return t.root
}

// Returns all the edges of the tree (do it recursively)
func (t *Tree) Edges() []*Edge {
	edges := make([]*Edge, 0, 2000)
	for _, e := range t.Root().br {
		edges = append(edges, e)
		t.edgesRecur(e, &edges)
	}
	return edges
}

func (t *Tree) edgesRecur(edge *Edge, edges *[]*Edge) {
	if len(edge.right.neigh) > 1 {
		for _, child := range edge.right.br {
			if child.left == edge.right {
				*edges = append((*edges), child)
				t.edgesRecur(child, edges)
			}
		}
	}
}

// Returns all the nodes of the tree (do it recursively)
func (t *Tree) Nodes() []*Node {
	nodes := make([]*Node, 0, 2000)
	t.nodesRecur(&nodes, nil, nil)
	return nodes
}

func (t *Tree) nodesRecur(nodes *[]*Node, cur *Node, prev *Node) {
	if cur == nil {
		cur = t.Root()
	}
	*nodes = append((*nodes), cur)
	for _, n := range cur.neigh {
		if n != prev {
			t.nodesRecur(nodes, n, cur)
		}
	}
}

// Removes a set of tips from the tree, from tip names
func (t *Tree) RemoveTips(names ...string) error {
	nodes := t.Nodes()
	nodeindex := nodeindex.New(t)

	for _, name := range names {
		n, ok := nodeindex.GetNode(name)
		if !ok {
			return errors.New("No tip named " + name + " in the Tree")
		}
		if len(n.neigh) != 1 {
			return errors.New("The node named " + name + " is not a tip")
		}
		if err := t.removeTip(n); err != nil {
			return err
		}
	}

	return nil
}

// Remove one tip from the tree
func (t *Tree) removeTip(tip *Node) error {
	if len(tip.neigh) != 1 {
		return errors.New("Cannot remove node, it is not a tip")
	}
	tip.neigh = nil
	internal := tip.br[0].left
	if err := internal.delNeighbor(tip); err != nil {
		return err
	}
	tip.neigh = nil
	tip.br[0].left = nil
	tip.br[0].right = nil
	tip.br = nil

	// Then 2 solutions :
	// 1 - Internal node is now terminal : it means it was the root of a rooted tree : we delete it and new root is its child
	// 2 - Internal node is now a bifurcation : we do not want to keep it thus we will delete it and connect the two neighbors
	// Case 1
	if len(internal.neigh) == 1 {
		if t.Root() != internal {
			return errors.New("After tip removal, this node should not have degre 1 without being the root")
		}
		t.root = internal.neigh[0]
		if err := t.root.delNeighbor(internal); err != nil {
			return err
		}
		t.delNode(internal)
		return nil
	}

	// Case 2: We remove the node
	if len(internal.neigh) == 2 {
		n1, n2 := internal.neigh[0], internal.neigh[1]
		b1, b2 := internal.br[0], internal.br[1]
		length1, length2 := b1.Length(), b2.Length()
		sup1, sup2 := b1.Support(), b2.Support()
		var e *Edge
		// Direction : true if n1-->internal
		dir1 := b1.left == n1
		// Direction : true if internal-->n2
		dir2 := b2.right == n2
		if err := n1.delNeighbor(internal); err != nil {
			return err
		}
		if err := n2.delNeighbor(internal); err != nil {
			return err
		}

		// Now we have two options to connect n1 and n2: (n1 parent of n2) or (n2 parent of n1)
		// This direction depends on the directions of the previous edges:
		// 1) n1--->internal--->n2 : t.ConnectNodes(n1,n2)
		// 2) n1<---internal<---n2 : t.ConnectNodes(n2,n1)
		// 3) n1<---internal--->n2 : internal is the root of an unrooted tree so:
		//        1 - we connect the two nodes from n1 to n2 if n1 is not a tip or n2 to n1 otherwise
		//        2 - we choose a new root (n1 if n1->n2, n2 otherwise)
		// 4) n1--->internal<---n2 : Error
		if dir1 && dir2 { // 1)
			e = t.ConnectNodes(n1, n2)
		} else if !dir1 && !dir2 { // 2)
			e = t.ConnectNodes(n2, n1)
		} else if !dir1 && dir2 { // 3
			if t.Root() != internal {
				return errors.New("The tree root is not the internal node, but it should be")
			}
			if len(n1.neigh) > 1 { // Not a tip
				e = t.ConnectNodes(n1, n2)
				t.SetRoot(n1)
			} else if len(n1.neigh) == 1 {
				return errors.New("The neighbor n1 should not have only one neighbor")
			} else if len(n2.neigh) > 1 { // Not a tip
				e = t.ConnectNodes(n2, n1)
				t.SetRoot(n2)
			} else if len(n2.neigh) == 1 {
				return errors.New("The neighbor n2 should not have only one neighbor")
			} else {
				return errors.New("The tree after tip removal is only made of two tips")
			}
		} else {
			return errors.New("Branches of internal node are not oriented as they should be")
		}

		if length1 != -1 || length2 != -1 {
			e.SetLength(math.Max(0, length1) + math.Max(0, length2))
		}

		// We attribute a support to the new branch only if it is not a tip branch
		if (sup1 != -1 || sup2 != -1) && len(n1.neigh) > 1 && len(n2.neigh) > 1 {
			e.SetSupport(math.Max(sup1, sup2))
		}

		t.delNode(internal)
		return nil
	}
	return errors.New("Unknown problem: The internal node remaining after removing the tip has a unexpected number of neighbors")
}

func (t *Tree) String() string {
	return t.Newick()
}

func (t *Tree) Newick() string {
	var buffer bytes.Buffer
	t.root.Newick(nil, &buffer)
	buffer.WriteString(";")
	return buffer.String()
}

func (t *Tree) UpdateTipIndex() {
	names := t.AllTipNames()
	sort.Strings(names)
	for k := range t.tipIndex {
		delete(t.tipIndex, k)
	}
	for i, n := range names {
		t.tipIndex[n] = uint(i)
	}
}

// if UpdateTipIndex has been called before ok
// otherwise returns an error
func (t *Tree) NbTips() (int, error) {
	if len(t.tipIndex) == 0 {
		return 0, errors.New("No tips in the index, tip name index is not initialized")
	}

	return len(t.tipIndex), nil

}

// Returns the bitset index of the tree in the Tree
// Returns an error if the node is not a tip
func (t *Tree) tipIndexNode(n *Node) (uint, error) {
	if len(n.neigh) != 1 {
		return 0, errors.New("Cannot get bitset index of a non tip node")
	}
	return t.TipIndex(n.name)
}

func (t *Tree) TipIndex(name string) (uint, error) {
	if len(t.tipIndex) == 0 {
		return 0, errors.New("No tips in the index, tip name index is not initialized")
	}
	v, ok := t.tipIndex[name]
	if !ok {
		return 0, errors.New("No tip named " + name + " in the index")
	}
	return v, nil
}

// Returns all the tip name in the tree
// Starts with n==nil (root)
func (t *Tree) AllTipNames() []string {
	names := make([]string, 0, 1000)
	t.allTipNamesRecur(&names, nil, nil)
	return names
}

// Returns all the tip name in the tree
// Starts with n==nil (root)
// It is an internal recursive function
func (t *Tree) allTipNamesRecur(names *[]string, n *Node, parent *Node) {
	if n == nil {
		n = t.Root()
	}
	// is a tip
	if len(n.neigh) == 1 {
		*names = append(*names, n.name)
	} else {
		for _, child := range n.neigh {
			if child != parent {
				t.allTipNamesRecur(names, child, n)
			}
		}
	}
}

func (n *Node) EdgeIndex(e *Edge) (int, error) {
	for i := 0; i < len(n.br); i++ {
		if n.br[i] == e {
			return i, nil
		}
	}
	return -1, errors.New("The Edge is not in the neighbors of node")
}

func (n *Node) NodeIndex(next *Node) (int, error) {
	for i := 0; i < len(n.neigh); i++ {
		if n.neigh[i] == next {
			return i, nil
		}
	}
	return -1, errors.New("The Node is not in the neighbors of node")
}

func (t *Tree) ConnectNodes(parent *Node, child *Node) *Edge {
	newedge := t.NewEdge()
	newedge.setLeft(parent)
	newedge.setRight(child)
	parent.addChild(child, newedge)
	child.addChild(parent, newedge)
	return newedge
}

// This function takes the first node having 3 neighbors
// and reroot the tree on this node
func (t *Tree) RerootFirst() error {
	for _, n := range t.Nodes() {
		if len(n.neigh) == 3 {
			err := t.Reroot(n)
			return err
		}
	}
	return errors.New("No nodes with 3 neighors have been found for rerooting")
}

func (t *Tree) ClearBitSets() error {
	length := uint(len(t.tipIndex))
	if length == 0 {
		return errors.New("No tips in the index, tip name index is not initialized")
	}
	t.clearBitSetsRecur(nil, nil, length)
	return nil
}

// Recursively update bitsets of edges from the Node n
// If node == nil then it starts from the root
func (t *Tree) clearBitSetsRecur(n *Node, parent *Node, ntip uint) {
	if n == nil {
		n = t.Root()
	}

	for i, child := range n.neigh {
		if child != parent {
			e := n.br[i]
			e.bitset = nil
			e.bitset = bitset.New(ntip)
			t.clearBitSetsRecur(child, n, ntip)
		}
	}
}

// Updates bitsets of all edges in the tree
// Assumes that the hashmap tip name : index is
// initialized with UpdateTipIndex function
func (t *Tree) UpdateBitSet() error {
	rightedges := make([]*Edge, 0, 2000)
	for _, e := range t.Root().br {
		rightedges = rightedges[:0]
		rightedges = append(rightedges, e)
		err := t.fillRightBitSet(e, &rightedges)
		if err != nil {
			return err
		}
	}
	return nil
}

// Recursively clears and sets the bitsets of the descending edges
//
func (t *Tree) fillRightBitSet(currentEdge *Edge, rightEdges *[]*Edge) error {
	if currentEdge.bitset == nil {
		return errors.New("BitSets has not been initialized with tree.clearBitSetsRecur(nil, nil, uint(len(tree.tipIndex)))")
	}
	currentEdge.bitset.ClearAll()
	// If we are at a tip edge
	// We set at 1 the bits of the tip in
	// the bitsets of all rightEdges
	if len(currentEdge.right.neigh) == 1 {
		i, err := t.tipIndexNode(currentEdge.right)
		if err != nil {
			return err
		}
		for _, e := range *rightEdges {
			e.bitset.Set(i)
		}
	} else {
		// Else
		for _, e2 := range currentEdge.right.br {
			if e2.left == currentEdge.right {
				*rightEdges = append(*rightEdges, e2)
				err := t.fillRightBitSet(e2, rightEdges)
				if err != nil {
					return err
				}
				*rightEdges = (*rightEdges)[:len(*rightEdges)-1]
			}
		}
	}
	return nil
}

// This function compares 2 trees and output
// the number of edges in common
// If the trees have different sets of tip names, returns an error
// It assumes that functions
// 	tree.UpdateTipIndex()
//	tree.ClearBitSets()
//	tree.UpdateBitSet()
// If tipedges is false: does not take into account tip edges
// Have been called before, otherwise will output an error
func (t *Tree) CommonEdges(t2 *Tree, tipEdges bool) (tree1 int, common int, tree2 int, err error) {

	err = t.CompareTipIndexes(t2)

	if err != nil {
		return 0, 0, 0, err
	}

	edges1 := t.Edges()
	edges2 := t2.Edges()

	tree1, common, tree2, err = CommonEdges(edges1, edges2)
	if !tipEdges {
		var nbtips int
		if nbtips, err = t.NbTips(); err != nil {
			return 0, 0, 0, err
		}
		common -= nbtips
	}

	return tree1, common, tree2, nil
}

// This function compares 2 trees and output
// the number of edges in common
// If the trees have different sets of tip names, returns an error
// It assumes that functions
// 	tree.UpdateTipIndex()
//	tree.ClearBitSets()
//	tree.UpdateBitSet()
// If tipedges is false: does not take into account tip edges
// Have been called before, otherwise will output an error
func CommonEdges(edges1 []*Edge, edges2 []*Edge) (tree1 int, common int, tree2 int, err error) {
	var e, e2 *Edge
	for _, e = range edges1 {
		if e.bitset == nil {
			return 0, 0, 0, errors.New("BitSets has not been initialized with tree.clearBitSetsRecur(nil, nil, uint(len(tree.tipIndex)))")
		}
		// if e.bitset.None() {
		// 	return 0, 0, 0, errors.New("One edge has a bitset of 0...000 : May be BitSets have not been updated with tree.UpdateBitSet()?")
		// }
		for _, e2 = range edges2 {
			if e2.bitset == nil {
				return 0, 0, 0, errors.New("BitSets has not been initialized with tree.clearBitSetsRecur(nil, nil, uint(len(tree.tipIndex)))")
			}
			if e.bitset.EqualOrComplement(e2.bitset) {
				common++
				// if e2.bitset.None() {
				// 	return 0, 0, 0, errors.New("One edge has a bitset of 0...000 : May be BitSets have not been updated with tree.UpdateBitSet()?")
				// }
				break
			}
		}
	}
	tree1 = len(edges1) - common
	tree2 = len(edges2) - common
	return tree1, common, tree2, nil
}

// This function compares this tree with a set of compTrees and outputs:
// 1) The number of edges specific to this tree for each comparison
// 2) The number of edges in common between this tree and each comp tree
// 3) The number of edges specific to all comp trees
// If tipedges is false: does not take into account tip edges
// If the trees have different sets of tip names, returns an error
// It assumes that functions
// 	tree.UpdateTipIndex()
//	tree.ClearBitSets()
//	tree.UpdateBitSet()
// Have been called before, otherwise will output an error
func (t *Tree) CompareEdges(compTrees []*Tree, tipEdges bool) (refTreeEdges []int, commonEdges []int, compTreeEdges []int, err error) {
	var tree1, common, tree2 int
	commonEdges = make([]int, len(compTrees))
	refTreeEdges = make([]int, len(compTrees))
	compTreeEdges = make([]int, len(compTrees))

	edges1 := t.Edges()
	for i, comp := range compTrees {
		if err = t.CompareTipIndexes(comp); err != nil {
			return nil, nil, nil, err
		}
		edges2 := comp.Edges()
		tree1, common, tree2, err = CommonEdges(edges1, edges2)
		if !tipEdges {
			var nbtips int
			if nbtips, err = t.NbTips(); err != nil {
				return nil, nil, nil, err
			}
			common -= nbtips
		}
		refTreeEdges[i] = tree1
		commonEdges[i] = common
		compTreeEdges[i] = tree2
	}
	return refTreeEdges, commonEdges, compTreeEdges, nil
}

// This function compares the tip name indexes of 2 trees
// If the tipindexes have the same size (!=0) and have the same set of tip names,
// The returns nil, otherwise returns an error
func (t *Tree) CompareTipIndexes(t2 *Tree) error {
	if len(t.tipIndex) == 0 ||
		len(t2.tipIndex) == 0 ||
		len(t.tipIndex) != len(t2.tipIndex) {
		return errors.New("Tip name index is not initialized or trees do not have the same number of tips")
	}

	for k := range t.tipIndex {
		_, ok := t2.tipIndex[k]
		if !ok {
			return errors.New("Trees do not have the same tip names")
		}
	}

	for k := range t2.tipIndex {
		_, ok := t.tipIndex[k]
		if !ok {
			return errors.New("Trees do not have the same tip names")
		}
	}
	return nil
}

// This function takes a node and reroot the tree on that node
// It reorients edges left-edge-right : see ReorderEdges
// The node must be one of the tree nodes, otherwise it returns an error
func (t *Tree) Reroot(n *Node) error {
	intree := false
	for _, n2 := range t.Nodes() {
		if n2 == n {
			intree = true
		}
	}
	if !intree {
		return errors.New("The node is not part of the tree")
	}
	t.root = n
	err := t.reorderEdges(n, nil)
	return err
}

// This function reorders the edges of a tree
// in order to always have left-edge-right
// with left node being parent of right node
// with respect to the given root node
// Important even for unrooted trees
// Useful mainly after a reroot
func (t *Tree) reorderEdges(n *Node, prev *Node) error {
	for _, next := range n.br {
		if next.right != prev && next.left != prev {
			if next.right == n {
				next.right, next.left = next.left, next.right
			}
			t.reorderEdges(next.right, n)
		}
	}
	return nil
}

// This function graft the Node n at the middle of the Edge e
// It divides the branch lenght by 2
// It returns the added edges and the added nodes
func (t *Tree) GraftTipOnEdge(n *Node, e *Edge) (*Edge, *Edge, *Node, error) {
	newnode := t.NewNode()
	newedge := t.NewEdge()

	lnode := e.left
	rnode := e.right

	// index of edge in neighbors of l
	e_l_ind, err := lnode.EdgeIndex(e)
	if err != nil {
		return nil, nil, nil, err
	}
	// index of edge in neighbors of r
	e_r_ind, err2 := rnode.EdgeIndex(e)
	if err2 != nil {
		return nil, nil, nil, err2
	}

	newedge.SetLength(1.0)
	newedge.setLeft(newnode)
	newedge.setRight(n)
	newnode.addChild(n, newedge)
	n.addChild(newnode, newedge)
	e.setRight(newnode)
	newnode.addChild(lnode, e)
	lnode.neigh[e_l_ind] = newnode

	if lnode.br[e_l_ind] != e {
		return nil, nil, nil, errors.New("The Edge is not at the same index")
	}

	newedge2 := t.NewEdge()
	newedge2.SetLength(e.length / 2)
	e.SetLength(e.length / 2)
	newedge2.setLeft(newnode)
	newedge2.setRight(rnode)
	newnode.addChild(rnode, newedge2)
	if rnode.br[e_r_ind] != e {
		return nil, nil, nil, errors.New("The Edge is not at the same index")
	}
	rnode.neigh[e_r_ind] = newnode
	rnode.br[e_r_ind] = newedge2
	return newedge, newedge2, newnode, nil
}

//Creates a Random Binary tree
//nbtips : Number of tips of the random binary tree to create
func RandomBinaryTree(nbtips int) (*Tree, error) {
	t := NewTree()
	if nbtips < 2 {
		return nil, errors.New("Cannot create a random binary tree with less than 2 tips")
	}
	edges := make([]*Edge, 0, 2000)
	for i := 1; i < nbtips; i++ {
		n := t.NewNode()
		n.SetName("Tip" + strconv.Itoa(i))
		switch len(edges) {
		case 0:
			n2 := t.NewNode()
			n2.SetName("Node" + strconv.Itoa(i-1))
			e := t.ConnectNodes(n2, n)
			edges = append(edges, e)
			e.SetLength(1.0)
			t.SetRoot(n2)
		default:
			// Where to insert the new tip
			i_edge := rand.Intn(len(edges))
			e := edges[i_edge]
			newedge, newedge2, _, err := t.GraftTipOnEdge(n, e)

			edges = append(edges, newedge)
			edges = append(edges, newedge2)

			if err != nil {
				return nil, err
			}
		}
	}
	err := t.RerootFirst()
	t.UpdateTipIndex()
	t.ClearBitSets()
	t.UpdateBitSet()

	return t, err
}

// Recursive function that outputs newick representation
// from the current node
func (n *Node) Newick(parent *Node, newick *bytes.Buffer) {
	if len(n.neigh) > 0 {
		if len(n.neigh) > 1 {
			newick.WriteString("(")
		}
		nbchild := 0
		for i, child := range n.neigh {
			if child != parent {
				if nbchild > 0 {
					newick.WriteString(",")
				}
				child.Newick(n, newick)
				if n.br[i].support != -1 {
					newick.WriteString(strconv.FormatFloat(n.br[i].support, 'f', 5, 64))
				}
				if len(child.comment) != 0 {
					for _, c := range child.comment {
						newick.WriteString("[")
						newick.WriteString(c)
						newick.WriteString("]")
					}
				}
				if n.br[i].length != -1 {
					newick.WriteString(":")
					newick.WriteString(strconv.FormatFloat(n.br[i].length, 'f', 5, 64))
				}
				nbchild++
			}
		}
		if len(n.neigh) > 1 {
			newick.WriteString(")")
		}
	}
	newick.WriteString(n.name)
}
