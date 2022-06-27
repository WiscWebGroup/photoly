import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {Heading, HStack***REMOVED*** from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";

export default function Home() {

    return (
        <>
            <Navbar></Navbar>
            <HStack>
                <Sidebar/>
                <Pathbar></Pathbar>
            </HStack>
        </>

    )
***REMOVED***