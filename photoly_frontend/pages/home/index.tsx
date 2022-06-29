import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {HStack***REMOVED*** from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";

export default function Home() {

    return (
        <>
            <Navbar></Navbar>
            <HStack alignItems={"flex-start"***REMOVED***>
                <Sidebar/>
                <Pathbar></Pathbar>
            </HStack>
        </>

    )
***REMOVED***