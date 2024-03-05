// This is created by https://gitlab.com/IvanSanchez/Leaflet.RepeatedMarkers
// Special Thanks to Iván Sánchez Ortega

L.GridLayer.RepeatedMarkers = L.GridLayer.extend({
	options: {
		// maxNativeZoom, noWrap and keepBuffer should **never** be changed
		// by users of this plugin.
		maxNativeZoom: 0,
		noWrap: true,
		keepBuffer: 0,

		maxZoom: Infinity,
	},

	initialize: function(options) {
		// A plain array to hold the original markers.
		this._masterMarkers = [];

		// A map of tile keys to arrays of markers.
		this._markersByTile = {};

		// A map of tile keys to longitude offsets
		this._offsetsByTile = {};
	},

	createTile: function(coords) {
		var key = this._tileCoordsToKey(coords);
		var longitudeOffset = coords.x * 360;

		this._markersByTile[key] = [];
		this._offsetsByTile[key] = longitudeOffset;

		/// TODO: iterate through this._masterMarkers and add each to the
		/// newly created this._markersByTile

		for (var i = 0, l = this._masterMarkers.length; i < l; i++) {
			this._markersByTile[key].push(
				this._addOffsetMarker(this._masterMarkers[i], longitudeOffset)
			);
		}

		return L.DomUtil.create("div");
	},

	_removeTile: function(key) {
		var markersToRemove = this._markersByTile[key];

		for (var i = 0, l = markersToRemove.length; i < l; i++) {
			if (this._map && markersToRemove[i]) {
				markersToRemove[i].removeFrom(this._map);
			}
		}

		delete this._markersByTile[key];
		delete this._offsetsByTile[key];

		return L.GridLayer.prototype._removeTile.call(this, key);
	},

	_addOffsetMarker(marker, longitudeOffset) {
		var originalLatLng = marker.getLatLng();
		var offsetLatLng = L.latLng(
			originalLatLng.lat,
			originalLatLng.lng + longitudeOffset
		);
		var copyMarker = L.marker(offsetLatLng, marker.options);

		// Copy the event handlers from the original marker.
		// This is a hard hack, going deep into the implementation of L.Evented
		for (var evName in marker._events) {
			marker._events[evName].forEach(function(i) {
				copyMarker.on(evName, i.fn, i.ctx || copyMarker);
			});
		}
		// Copy a reference to the bound popup, if any
		copyMarker._popup = marker._popup;

		if (this._map) {
			this._map.addLayer(copyMarker);
		}
		return copyMarker;
	},

	addMarker(marker) {
		if (!(marker instanceof L.Marker)) {
			throw new Error(
				"L.GridLayer.RepeatedMarkers can only hold instances of L.Marker."
			);
		}

		// Adds the marker to the reference group, **and** to **each** of
		// the this._markersByTile arrays (and each time, add its copy to
		// the map)

		this._masterMarkers.push(marker);

		for (var key in this._markersByTile) {
			this._markersByTile[key][L.stamp(marker)] = this._addOffsetMarker(
				marker,
				this._offsetsByTile[key]
			);
		}
	},

	removeMarker(marker) {
		var i = this._masterMarkers.indexOf(marker);
		if (i === -1) {
			return false;
		}

		var masterMarkerId = L.stamp(marker);

		for (var key in this._markersByTile) {
			if (this._map) {
				this._markersByTile[key][masterMarkerId].remove();
			}
			delete this._markersByTile[key][masterMarkerId];
		}
	},
});

L.gridLayer.repeatedMarkers = function(opts) {
	return new L.GridLayer.RepeatedMarkers(opts);
};