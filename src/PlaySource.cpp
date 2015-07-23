#include "PlaySource.h"

#include <stdlib.h>

void PlaySource::run() {

	skrillex::ResultSet<skrillex::Song> queue;
	m_db->getQueue(queue);

	for (auto& song : queue) {

		//TEMP
		printf("%s: %s (%s)", song.artist.name.c_str(), song.name.c_str(), song.genre.name.c_str());
		// END TEMP

		// Song finished
		m_db->songFinished();
	}

}
