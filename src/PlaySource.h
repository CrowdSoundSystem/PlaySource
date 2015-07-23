#ifndef PlaySource_HEADER
#define PlaySource_HEADER

#include <memory>

// DB stuff
#include "skrillex/skrillex.hpp"

class PlaySource {

public:

	PlaySource(std::shared_ptr<skrillex::DB> db)
		: m_db(db)
	{}

	void run();

private:

	std::shared_ptr<skrillex::DB> m_db;

};

#endif