import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {HStack***REMOVED*** from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";
import Images from "../../components/Images";
import React from "react";

const Home: React.FC = () => {

    return (
        <>
            <Navbar></Navbar>
            <HStack alignItems={"flex-start"***REMOVED***>
                <Sidebar/>
                <Pathbar>
                    <Images/>
                </Pathbar>
            </HStack>
        </>

    )
***REMOVED***
export default Home