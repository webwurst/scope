const dagre = require('dagre');
const debug = require('debug')('scope:nodes-layout');
// const Naming = require('../constants/naming');
const _ = require('lodash');

const MAX_NODES = 100;
const topologyGraphs = {};

const doLayout = function(nodes, edges, width, height, scale, margins, topologyId) {
  let offsetX = 0 + margins.left;
  let offsetY = 0 + margins.top;
  let graph;

  if (_.size(nodes) > MAX_NODES) {
    debug('Too many nodes for graph layout engine. Limit: ' + MAX_NODES);
    return null;
  }

  // one engine per topology, to keep renderings similar
  if (!topologyGraphs[topologyId]) {
    topologyGraphs[topologyId] = new dagre.graphlib.Graph({});
  }
  graph = topologyGraphs[topologyId];

  // configure node margins
  graph.setGraph({
    // rankdir: 'LR',
    nodesep: scale(3),
    ranksep: scale(5)
  });

  // create rank nodes
  const nodesByRank = _.groupBy(nodes, function(node) {
    return node.rank;
  });
  const rankNodePrefix = 'scope-rank-';

  _.each(nodesByRank, function(rankNodes, rank) {
    if (rank) {
      // nodes with rank are clustered in one big node in layout
      const nodeId = rankNodePrefix + rank;
      if (!graph.hasNode(nodeId)) {
        graph.setNode(nodeId, {
          id: nodeId,
          width: scale(Math.sqrt(rankNodes.length)),
          height: scale(Math.sqrt(rankNodes.length)),
          nodes: rankNodes
        });
      }
      // store rankNode reference in rank nodes
      _.each(rankNodes, function(node) {
        node.rankNode = nodeId;
      });
    } else {
      // nodes with no rank get layed out individually
      _.each(rankNodes, function(node) {
        if (!graph.hasNode(node.id)) {
          graph.setNode(node.id, {
            id: node.id,
            width: scale(1),
            height: scale(1)
          });
        }
      });
    }
  });

  // TODO remove rank and non-rank nodes

  // remove nodes that are no longer there
  // _.each(graph.nodes(), function(nodeid) {
  //   if (!_.has(nodes, nodeid)) {
  //     graph.removeNode(nodeid);
  //   }
  // });

  // add edges to the graph if not already there
  _.each(edges, function(edge) {
    const sourceId = edge.source.rankNode || edge.source.id;
    const targetId = edge.target.rankNode || edge.target.id;
    if (!graph.hasEdge(sourceId, targetId)) {
      graph.setEdge(sourceId, targetId, {id: edge.id});
    }
  });

  // TODO remoed egdes that are no longer there
  // _.each(graph.edges(), function(edgeObj) {
  //   const edge = [edgeObj.v, edgeObj.w];
  //   const edgeId = edge.join(Naming.EDGE_ID_SEPARATOR);
  //   if (!_.has(edges, edgeId)) {
  //     graph.removeEdge(edgeObj.v, edgeObj.w);
  //   }
  // });

  dagre.layout(graph);

  const layout = graph.graph();

  // shifting graph coordinates to center

  if (layout.width < width) {
    offsetX = (width - layout.width) / 2 + margins.left;
  }
  if (layout.height < height) {
    offsetY = (height - layout.height) / 2 + margins.top;
  }

  // apply coordinates to nodes and edges

  graph.nodes().forEach(function(id) {
    const graphNode = graph.node(id);
    if (graphNode.nodes) {
      // cluster node
      const centerX = graphNode.x;
      const centerY = graphNode.y;
      const clusterCount = graphNode.nodes.length;
      const radius = scale(Math.sqrt(clusterCount));
      _.each(graphNode.nodes, function(node, i) {
        const angle = Math.PI * 2 * i / clusterCount;
        node.x = centerX + radius * Math.sin(angle);
        node.y = centerY + radius * Math.cos(angle);
      });
    } else {
      // normal node
      const node = nodes[id];
      node.x = graphNode.x + offsetX;
      node.y = graphNode.y + offsetY;
    }
  });

  _.each(edges, function(edge) {
    if (!edge.points) {
      edge.points = [
        {x: edge.source.x, y: edge.source.y},
        {x: edge.target.x, y: edge.target.y}
      ];
    }
  });

  // return object with the width and height of layout

  return layout;
};

module.exports = {
  doLayout: doLayout
};
