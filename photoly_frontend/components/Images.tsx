import {Box, Flex, Image***REMOVED*** from "@chakra-ui/react";
import {ReactNode***REMOVED*** from "react";
import ImageContextMenu from "./ImageContextMenu";
import {useContextMenu***REMOVED*** from "../hooks/useContextMenu";

export default function Images(): ReactNode{
    const [showContext, handleContextMenu, anchorPoint] = useContextMenu();
    return (
        <>
            <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={handleContextMenu***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={handleContextMenu***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg'/>
                </Box>
            </Flex>
        ***REMOVED***showContext && <ImageContextMenu x={anchorPoint.x***REMOVED*** y={anchorPoint.y***REMOVED***/>***REMOVED***
        </>

    )
***REMOVED***