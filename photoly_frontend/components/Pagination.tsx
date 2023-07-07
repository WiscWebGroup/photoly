import { ArrowLeftIcon, ArrowRightIcon, ChevronLeftIcon, ChevronRightIcon } from '@chakra-ui/icons';
import { HStack, Box, Button, IconButton, VStack, Flex, NumberDecrementStepper, NumberIncrementStepper, NumberInput, NumberInputField, NumberInputStepper, Select, Tooltip, Text } from '@chakra-ui/react';
import { stringify } from 'querystring';
import React, { useEffect } from 'react'

interface pageInfo{
    rowsPerPage:number;
    totalPage:number;
    page: number;
    changeSelection: (page: number) => void;
}

const Pagination = ({rowsPerPage, totalPage, page, changeSelection} : pageInfo) => {

    const [canPreviousPage, setCanPreviousPage] = React.useState<boolean>(false);
    const [canNextPage, setCanNextPage] = React.useState<boolean>(false);

    useEffect(() =>{
        if (page <= 1)
        {
            setCanPreviousPage(false)
        }else {
            setCanPreviousPage(true)
        }

        if (page >= totalPage)
        {
            setCanNextPage(false)
        }else {
            setCanNextPage(true)
        }
    }, [rowsPerPage, totalPage, page])


  return (
    <Flex justifyContent="space-between" m={4} alignSelf="center" w={"50%"} mt={5}>
        <Flex>
          <Tooltip label="First Page">
            <IconButton
                      onClick={() => changeSelection(1)}
                      isDisabled={!canPreviousPage}
                      icon={<ArrowLeftIcon h={3} w={3} />}
                      mr={4} aria-label={''}            />
          </Tooltip>
          <Tooltip label="Previous Page">
            <IconButton
                      onClick={()=>{changeSelection(page-1)}}
                      isDisabled={!canPreviousPage}
                      icon={<ChevronLeftIcon h={6} w={6} />} aria-label={''}            />
          </Tooltip>
        </Flex>

        <Flex alignItems="center">
          <Text flexShrink="0" mr={8}>
            Page{" "}
            <Text fontWeight="bold" as="span">
              {page}
            </Text>{" "}
            of{" "}
            <Text fontWeight="bold" as="span">
              {totalPage}
            </Text>
          </Text>
          <Text flexShrink="0">Go to page:</Text>{" "}
          <NumberInput
            ml={2}
            mr={8}
            w={28}
            min={1}
            max={totalPage}
            onChange={(value) => {
              changeSelection(parseInt(value));
            }}
            defaultValue={page}
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
                      onClick={()=>{changeSelection(page+1)}}
                      isDisabled={!canNextPage}
                      icon={<ChevronRightIcon h={6} w={6} />} aria-label={''}            />
          </Tooltip>
          <Tooltip label="Last Page">
            <IconButton
                      onClick={() => changeSelection(totalPage)}
                      isDisabled={!canNextPage}
                      icon={<ArrowRightIcon h={3} w={3} />}
                      ml={4} aria-label={''}            />
          </Tooltip>
        </Flex>
      </Flex>
    
  )
}

export default Pagination
