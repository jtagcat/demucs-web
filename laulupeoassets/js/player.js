function fancyTimeFormat(duration) {
	// Hours, minutes and seconds
	var hrs = ~~(duration / 3600);
	var mins = ~~((duration % 3600) / 60);
	var secs = ~~duration % 60;

	// Output like "1:01" or "4:03:59" or "123:03:59"
	var ret = "";

	if (hrs > 0) {
		ret += "" + hrs + ":" + (mins < 10 ? "0" : "");
	}

	ret += "" + mins + ":" + (secs < 10 ? "0" : "");
	ret += "" + secs;
	return ret;
}

var total_tracks = tracks.length;
var loaded_tracks = 0;

var options = {
	theme: "sk-circle",
	message: "Loading audio files...<br />Loaded: "+loaded_tracks+"/"+total_tracks,
};

function percentConverter(value) {
	return Math.round(value * 100) / 100;
}

HoldOn.open(options);

$(function() {
	var playing = 0;

	console.table(tracks);
	console.table(bookmarks);
	console.table(presets);

	var sounds = [];

	presets.forEach(function(preset, key) {
		$(".presetsWrapper>.row").append('\
		<div class="col d-grid">\
			<button class="btn btn-primary preset btn-sm" data-preset="'+preset.id+'">'+preset.name+'</button>\
		</div>\
		');
	});

	bookmarks.forEach(function(bookmark) {
		$(".bookmarksWrapper").append('\
		<div class="row bookmark pointer ps-3" data-time="'+bookmark.time+'">\
			<div class="col-12">\
					▶️\
				'+fancyTimeFormat(bookmark.time)+'\ - '+bookmark.name+'\
			</div>\
		</div>\
		');
	});

	tracks.forEach(function(track, key) {
		key = track.id;

		track.minVolume = percentConverter(track.minVolume);
		track.maxVolume = percentConverter(track.maxVolume);
		track.defaultVolume = percentConverter(track.defaultVolume);
		
		sounds[key] = new Howl({
			src: [
				`/results/${track.src}`,
				// `${fileUrl}${track.id}.webm`,
				// `${fileUrl}${track.id}.mp3`,
				// fileUrl+track.file+".webm",
				// fileUrl+track.file+".mp3",
			],
			preload: true,
			html5: false,
			mute: track.mute,
			// volume: track.default,
		});

		if(Object.keys(sounds).length == 1) {
			console.log("First track, starting progressbar update");

			setInterval(() => {
				updateProgressbar();
			}, 300);

			function updateProgressbar() {
				if (sounds[key].playing()) {
					currentPosition = sounds[key].seek();

					duration = sounds[key].duration();
					$(".seekbarWrapper .duration").html(fancyTimeFormat(currentPosition) + "/" + fancyTimeFormat(duration));


					// console.log("Seek:", currentPosition, "Dur:", duration)

					$(".seekbarWrapper .seekbar").val(currentPosition).attr("min", 0).attr("max", duration);
				}
			}
		}

		console.log("Track lock:", track.locked);

		$(".faders>.row").append('\
			<div class="col text-center track" data-track="'+key+'">\
				<div class="row">\
					<div class="col-12 faderWrapper">\
						<input type="range" '+(track.locked == true ? "disabled" : "")+' data-min="'+track.minVolume+'" data-max="'+track.maxVolume+'" min="'+(track.minVolume/100).toFixed(2)+'" max="'+(track.maxVolume/100).toFixed(2)+'" step="0.01" value="'+(track.defaultVolume/100).toFixed(2)+'" class="form-range trackFader h-100" data-track="'+key+'" orient="vertical">\
					</div>\
				</div>\
				<div class="row">\
					<div class="col-12 text-muted volume d-none">\
						'+(track.defaultVolume*100)+'%\
					</div>\
				</div>\
				<div class="row">\
					<div class="col-12">\
						'+track.name+'\
					</div>\
				</div>\
				<div class="row mt-1">\
					<div class="col-12">\
						<button class="btn btn-secondary mute btn-sm '+(track.mute == true ? "collapse": "")+' '+(track.muteable == false ? "d-none" : "")+'" data-track="'+key+'">Mute</button>\
						<button class="btn btn-primary unmute btn-sm '+(track.mute == false ? "collapse": "")+'" data-track="'+key+'">Unmute</button>\
					</div>\
				</div>\
			</div>\
		');

		sounds[key].on('load', function(){
			console.log("Loading track");
			loaded_tracks++;

			$("#holdon-message").html("Loading audio files<br />Loaded: "+loaded_tracks+"/"+total_tracks);

			if(loaded_tracks == total_tracks) {
				console.log(`Track #${loaded_tracks} loaded!`);
				HoldOn.close();
				$("button").prop("disabled", false);
			}
		});
	});

	function stopAll() {
		if(playing == 1) {
			playing = 0;
			$("#pause").hide();
			$("#play").show();

			tracks.forEach(function(track, key) {
				sounds[track.id].stop()
			});

		}
	}

	function playAll() {
		if(playing == 0) {
			playing = 1;
			$("#pause").show();
			$("#play").hide();

			tracks.forEach(function(track, key) {
				sounds[track.id].play()
			});

		}
	}

	function pauseAll() {
		if(playing == 1) {
			playing = 0;
			$("#pause").hide();
			$("#play").show();

			tracks.forEach(function(track, key) {
				sounds[track.id].pause()
			});
		}
	}

	$(document).on("change", ".seekbarWrapper .seekbar" , function() {
		tracks.forEach(function(track, key) {
			sounds[track.id].seek($(".seekbarWrapper .seekbar").val())
		});
	});


	$(document).on("click", ".bookmarksWrapper .bookmark" , function() {
		var time = $(this).data("time");
		console.log("Bookmark click, time:", time);

		$(".seekbarWrapper .seekbar").val(time).change();

		playAll();
	});

	$(document).on("click", ".presetsWrapper .preset" , function() {
		var preset = $(this).data("preset");
		console.log("Preset click:", preset);

		var selectedPreset = presets.filter(function (data) { return data.id == preset });

		console.table(selectedPreset[0]["tracks"]);

		Object.keys(selectedPreset[0]["tracks"]).forEach(function(key) {
			var volume = percentConverter(selectedPreset[0]["tracks"][key]);
			console.log("Preset volume track", key, volume);

			$(".track[data-track='"+key+"'] .trackFader").val(volume/100).trigger("input");
			$(".track[data-track='"+key+"'] .volume").html(volume+"%");
		});

		playAll();
	});

	$(document).on("click", ".mute" , function() {
		var track = $(this).data("track");
		console.log(track, "Mute klikk");
		$(this).hide();
		$(".unmute[data-track='"+track+"']").show();

		sounds[track].mute(true);
	});

	$(document).on("click", ".unmute" , function() {
		var track = $(this).data("track");
		console.log(track, "Unmute klikk");
		$(this).hide();
		$(".mute[data-track='"+track+"']").show();

		sounds[track].mute(false);
	});

	$(document).on("input", ".trackFader" , function() {
		volume = $(this).val();
		track = $(this).data("track");
		console.log("Volume change RAW", track, volume, "min:", $(this).attr("min"), "max:", $(this).attr("max"));

		if(volume < $(this).attr("min")) {
			volume = $(this).attr("min");
			$(this).val(volume/100);
		}

		if(volume > $(this).attr("max")) {
			volume = $(this).attr("max");
			$(this).val(volume/100);
		}

		console.log("Volume change FILTER", track, volume);
		$(".track[data-track='"+track+"'] .volume").html(Math.floor(volume*100)+"%");

		sounds[track].volume(volume);
	});

	$("#stop").click(function() {
		stopAll();
	});

	$("#pause").click(function() {
		pauseAll();
	});

	$("#play").click(function() {
		playAll();
	});
});
