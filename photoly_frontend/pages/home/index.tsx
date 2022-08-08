import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {HStack***REMOVED*** from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";
import Images from "../../components/Images";
import React from "react";
import SearchContextProvider from "../../components/contexts/SearchContext";

const Home: React.FC = () => {
    return (
        <>
            <Navbar></Navbar>
            <HStack alignItems={"flex-start"***REMOVED***>
                <SearchContextProvider>
                    <Sidebar/>
                    <Pathbar>
                        <Images/>
                    </Pathbar>
                </SearchContextProvider>
            </HStack>
        </>

    )
***REMOVED***
export default Home