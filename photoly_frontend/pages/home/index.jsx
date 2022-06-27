import Navbar from "../../components/Navbar"
import Sidebar from "../../components/Sidebar";
import {HStack} from "@chakra-ui/react";
import Pathbar from "../../components/Pathbar";

export default function Home() {

    return (
        <>
            <Navbar></Navbar>
            <HStack alignItems={"flex-start"}>
                <Sidebar/>
                <Pathbar></Pathbar>
            </HStack>
        </>

    )
}