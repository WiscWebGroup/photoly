import { ArrowLeftIcon, ArrowRightIcon, ChevronLeftIcon, ChevronRightIcon ***REMOVED*** from '@chakra-ui/icons';
import { HStack, Box, Button, IconButton, VStack, Flex, NumberDecrementStepper, NumberIncrementStepper, NumberInput, NumberInputField, NumberInputStepper, Select, Tooltip, Text ***REMOVED*** from '@chakra-ui/react';
import { stringify ***REMOVED*** from 'querystring';
import React, { useEffect ***REMOVED*** from 'react'

interface pageInfo{
    rowsPerPage:number;
    totalPage:number;
    page: number;
    changeSelection: (page: number) => void;
***REMOVED***

const Pagination = ({rowsPerPage, totalPage, page, changeSelection***REMOVED*** : pageInfo) => {

    const [canPreviousPage, setCanPreviousPage] = React.useState<boolean>(false);
    const [canNextPage, setCanNextPage] = React.useState<boolean>(false);

    useEffect(() =>{
        if (page <= 1)
    ***REMOVED***
            setCanPreviousPage(false)
***REMOVED***else {
            setCanPreviousPage(true)
***REMOVED***

        if (page >= totalPage)
    ***REMOVED***
            setCanNextPage(false)
***REMOVED***else {
            setCanNextPage(true)
***REMOVED***
***REMOVED***, [rowsPerPage, totalPage, page])


  return (
    <Flex justifyContent="space-between" m={4***REMOVED*** alignSelf="center" w={"50%"***REMOVED*** mt={5***REMOVED***>
        <Flex>
          <Tooltip label="First Page">
            <IconButton
                      onClick={() => changeSelection(1)***REMOVED***
                      isDisabled={!canPreviousPage***REMOVED***
                      icon={<ArrowLeftIcon h={3***REMOVED*** w={3***REMOVED*** />***REMOVED***
                      mr={4***REMOVED*** aria-label={''***REMOVED***            />
          </Tooltip>
          <Tooltip label="Previous Page">
            <IconButton
                      onClick={()=>{changeSelection(page-1)***REMOVED******REMOVED***
                      isDisabled={!canPreviousPage***REMOVED***
                      icon={<ChevronLeftIcon h={6***REMOVED*** w={6***REMOVED*** />***REMOVED*** aria-label={''***REMOVED***            />
          </Tooltip>
        </Flex>

        <Flex alignItems="center">
          <Text flexShrink="0" mr={8***REMOVED***>
            Page{" "***REMOVED***
            <Text fontWeight="bold" as="span">
          ***REMOVED***page***REMOVED***
            </Text>{" "***REMOVED***
            of{" "***REMOVED***
            <Text fontWeight="bold" as="span">
          ***REMOVED***totalPage***REMOVED***
            </Text>
          </Text>
          <Text flexShrink="0">Go to page:</Text>{" "***REMOVED***
          <NumberInput
            ml={2***REMOVED***
            mr={8***REMOVED***
            w={28***REMOVED***
            min={1***REMOVED***
            max={totalPage***REMOVED***
            onChange={(value) => {
              changeSelection(parseInt(value));
    ***REMOVED******REMOVED***
            defaultValue={page***REMOVED***
          >
            <NumberInputField />
            <NumberInputStepper>
              <NumberIncrementStepper />
              <NumberDecrementStepper />
            </NumberInputStepper>
          </NumberInput>
        </Flex>

        <Flex>
          <Tooltip label="Next Page">
            <IconButton
                      onClick={()=>{changeSelection(page+1)***REMOVED******REMOVED***
                      isDisabled={!canNextPage***REMOVED***
                      icon={<ChevronRightIcon h={6***REMOVED*** w={6***REMOVED*** />***REMOVED*** aria-label={''***REMOVED***            />
          </Tooltip>
          <Tooltip label="Last Page">
            <IconButton
                      onClick={() => changeSelection(totalPage)***REMOVED***
                      isDisabled={!canNextPage***REMOVED***
                      icon={<ArrowRightIcon h={3***REMOVED*** w={3***REMOVED*** />***REMOVED***
                      ml={4***REMOVED*** aria-label={''***REMOVED***            />
          </Tooltip>
        </Flex>
      </Flex>
    
  )
***REMOVED***

export default Pagination
