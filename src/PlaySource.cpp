#include "PlaySource.h"

#include <thread>
#include <chrono>

void PlaySource::run() {

	skrillex::ResultSet<skrillex::Song> queue;
	m_db->getQueue(queue);

	for (auto& song : queue) {

		//TEMP
		printf("Playing: %s: %s (%s)\n", song.artist.name.c_str(), song.name.c_str(), song.genre.name.c_str());
        std::this_thread::sleep_for(std::chrono::seconds(5));
		// END TEMP

		// Song finished
		m_db->songFinished();
	}

}
