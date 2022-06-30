import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {HStack***REMOVED*** from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";
import Images from "../../components/Images";

export default function Home() {

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