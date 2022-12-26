import { AddIcon ***REMOVED*** from "@chakra-ui/icons"
import { Box, Button, ButtonGroup, FormControl, FormLabel, HStack, Icon, IconButton, Input, Popover, PopoverArrow, PopoverCloseButton, PopoverContent, PopoverTrigger, Stack, Text, useDisclosure ***REMOVED*** from "@chakra-ui/react"
import { ChangeEvent, ReactNode, useRef, useState ***REMOVED*** from "react"
import { IconType ***REMOVED*** from "react-icons"
import { useTagListUpdate ***REMOVED*** from "./contexts/TagContext"
import {useInsertTagMutation***REMOVED*** from "../redux/api/apiSlice";

interface TagHeaderProps {
    headerIcon?: IconType
    iconColor?: string
    children?: ReactNode
***REMOVED***

const TagHeader = ({headerIcon, iconColor, children***REMOVED***: TagHeaderProps) => {
    const inputRef = useRef(null)
    const [tag, setTag] = useState<string>("")
    const [isDisabled, setIsDisabled] = useState(true)  // control submit button
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()  // control popover
    const [insert, _] = useInsertTagMutation()

    const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const str = e.currentTarget.value
        setTag(str);
        setIsDisabled(str.length === 0)
***REMOVED***

    const handleClose = () => {
        setTag("")
        onClose()
***REMOVED***

    const handleInsert = () => {
        insert(tag)
        handleClose()
***REMOVED***

    return (
        <Box style={{width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center'***REMOVED******REMOVED***>
            <HStack>
                <Icon as={headerIcon***REMOVED*** color={iconColor***REMOVED*** />
                <Text>{children***REMOVED***</Text>
            </HStack>
            <Popover
                isOpen={isOpen***REMOVED***
                initialFocusRef={inputRef***REMOVED***
                onOpen={onOpen***REMOVED***
                onClose={handleClose***REMOVED***
                placement='auto'
                closeOnBlur={false***REMOVED***
            >
                <PopoverTrigger>
                    <IconButton variant="unstyled" size="xs" aria-label="Add tag" icon={<AddIcon />***REMOVED*** />
                </PopoverTrigger>
                <PopoverContent p={5***REMOVED***>
                    <PopoverArrow />
                    <PopoverCloseButton />
                    <Stack spacing={4***REMOVED***>
                        <FormControl>
                            <FormLabel>New Tag</FormLabel>
                            <Input size='sm' ref={inputRef***REMOVED*** onChange={handleChange***REMOVED*** />
                        </FormControl>
                        <ButtonGroup display='flex' justifyContent='flex-end'>
                            <Button size='sm' variant='outline' onClick={handleClose***REMOVED***>
                                Cancel
                            </Button>
                            <Button size='sm' isDisabled={isDisabled***REMOVED*** onClick={handleInsert***REMOVED*** colorScheme='teal'>
                                Save
                            </Button>
                        </ButtonGroup>
                    </Stack>
                </PopoverContent>
            </Popover>
        </Box>
    )
***REMOVED***

export default TagHeader