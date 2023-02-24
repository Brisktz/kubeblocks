/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dag

import (
	"errors"
	"fmt"
)

type DAG struct {
	vertices map[Vertex]Vertex
	edges    map[Edge]Edge
}

type Vertex interface {}

type Edge interface {
	From() Vertex
	To() Vertex
}

type realEdge struct {
	F, T Vertex
}

type WalkFunc func(v Vertex) error

func (d *DAG) AddVertex(v Vertex) bool {
	if v == nil {
		return false
	}
	d.vertices[v] = v
	return true
}

func (d *DAG) RemoveVertex(v Vertex) bool {
	if v == 0 {
		return true
	}
	for k := range d.edges {
		if k.From() == v || k.To() == v {
			delete(d.edges, k)
		}
	}
	return true
}

func (d *DAG) AddEdge(e Edge) bool {
	if e.From() == nil || e.To() == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			return true
		}
	}
	d.edges[e]= e
	return true
}

func (d *DAG) RemoveEdge(e Edge) bool {
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			delete(d.edges, k)
		}
	}
	return true
}

func (d *DAG) Connect(from, to Vertex) bool {
	if from == nil || to == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == from && k.To() == to {
			return true
		}
	}
	edge :=RealEdge(from, to)
	d.edges[edge] = edge
	return true
}

func (d *DAG) WalkTopoOrder(walkFunc WalkFunc) error {
	if err := d.validate(); err != nil {
		return err
	}
	orders := d.topologicalOrder(false)
	for _, v := range orders {
		if err := walkFunc(v); err != nil {
			return err
		}
	}
	return nil
}

func (d *DAG) WalkReverseTopoOrder(walkFunc WalkFunc) error {
	if err := d.validate(); err != nil {
		return err
	}
	orders := d.topologicalOrder(true)
	for _, v := range orders {
		if err := walkFunc(v); err != nil {
			return err
		}
	}
	return nil
}

// validate 'd' has single root and has no cycles
func (d *DAG) validate() error {
	// single root validation
	root := d.root()
	if root == nil {
		return errors.New("no single root found")
	}

	// self-cycle validation
	for e := range d.edges {
		if e.From() == e.To() {
			return fmt.Errorf("self-cycle found: %v", e.From())
		}
	}

	// cycle validation
	// use a DFS func to find cycles
	walked := make(map[Vertex]bool)
	marked := make(map[Vertex]bool)
	var walk func(v Vertex) error
	walk = func(v Vertex) error {
		if walked[v] {
			return nil
		}
		if marked[v] {
			return errors.New("cycle found")
		}

		marked[v] = true
		adjacent := d.outAdj(v)
		for _, vertex := range adjacent {
			walk(vertex)
		}
		marked[v] = false
		walked[v] = true
		return nil
	}
	for v := range d.vertices {
		if err := walk(v); err != nil {
			return err
		}
	}
	return nil
}

// topologicalOrder assumes 'd' is a legal DAG
func (d *DAG) topologicalOrder(reverse bool) []Vertex {
	// orders is what we want, a (reverse) topological order of this DAG
	orders := make([]Vertex, 0)

	// walked marks vertex has been walked, to stop recursive func call
	walked := make(map[Vertex]bool)

	// walk is a DFS func
	var walk func(v Vertex)
	walk = func(v Vertex) {
		if walked[v] {
			return
		}
		var adjacent []Vertex
		if reverse {
			adjacent = d.outAdj(v)
		} else {
			adjacent = d.inAdj(v)
		}
		for _, vertex := range adjacent {
			walk(vertex)
		}
		walked[v] = true
		orders = append(orders, v)
	}
	for v := range d.vertices {
		walk(v)
	}

	return orders
}

func (d *DAG) root() Vertex {
	roots := make([]Vertex, 0)
	for n := range d.vertices {
		if len(d.inAdj(n)) == 0 {
			roots = append(roots, n)
		}
	}
	if len(roots) != 1 {
		return nil
	}
	return roots[0]
}

// outAdj returns all adjacent vertices that v points to
func (d *DAG) outAdj(v Vertex) []Vertex {
	vertices := make([]Vertex, 0)
	for e := range d.edges {
		if e.From() == v {
			vertices = append(vertices, e.To())
		}
	}
	return vertices
}

// inAdj returns all adjacent vertices that point to v
func (d *DAG) inAdj(v Vertex) []Vertex {
	vertices := make([]Vertex, 0)
	for e := range d.edges {
		if e.To() == v {
			vertices = append(vertices, e.From())
		}
	}
	return vertices
}

func (r *realEdge) From() Vertex {
	return r.F
}

func (r *realEdge) To() Vertex {
	return r.T
}

func New() *DAG {
	dag := &DAG{
		vertices: make(map[Vertex]Vertex),
		edges:    make(map[Edge]Edge),
	}
	return dag
}

func RealEdge(from, to Vertex) Edge {
	return &realEdge{F: from, T: to}
}