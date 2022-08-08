import { AddIcon ***REMOVED*** from "@chakra-ui/icons"
import { Box, Button, ButtonGroup, FormControl, FormLabel, HStack, Icon, IconButton, Input, Popover, PopoverArrow, PopoverCloseButton, PopoverContent, PopoverTrigger, Select, Stack, Text, useDisclosure ***REMOVED*** from "@chakra-ui/react"
import { ChangeEvent, ReactNode, useRef, useState ***REMOVED*** from "react"
import { IconType ***REMOVED*** from "react-icons"
import { useAlbumListUpdate ***REMOVED*** from "./contexts/AlbumContext"

interface AlbumHeaderProps {
    headerIcon?: IconType
    iconColor?: string
    children?: ReactNode
***REMOVED***

const AlbumHeader = ({ headerIcon, iconColor, children ***REMOVED*** : AlbumHeaderProps) => {
    const inputRef = useRef(null)
    const [name, setName] = useState("")
    const [coverId, setCoverId] = useState(0)
    const [coverColor, setCovereColor] = useState("#FFF")
    const { insertRequest ***REMOVED*** = useAlbumListUpdate()
    const [isDisabled, setIsDisabled] = useState(true)  // control submit button
    const [isLoading, setIsLoading] = useState(false)
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()  // control popover

    const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const str = e.currentTarget.value
        setName(str);
        setIsDisabled(str.length === 0)
***REMOVED***

    const handleClose = () => {
        setName("")
        onClose()
***REMOVED***

    const handleInsert = () => {
        setIsLoading(true)
        insertRequest(name, coverId, coverColor)
        setIsLoading(false)
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
                            <FormLabel>New Album</FormLabel>
                            <Input size='sm' ref={inputRef***REMOVED*** onChange={handleChange***REMOVED*** />
                        </FormControl>
                        <FormControl>
                            <FormLabel>Select Icon</FormLabel>
                            <Select
                                placeholder="Icon Format"
                                mt={3***REMOVED***
                                onChange={(e) => setCoverId(parseInt(e.currentTarget.value))***REMOVED***
                            >
                                <option value={0***REMOVED***>Default</option>
                                <option value={1***REMOVED***>Star</option>
                                <option value={2***REMOVED***>Favorite</option>
                                <option value={3***REMOVED***>Beach</option>
                                <option value={4***REMOVED***>Plane</option>
                                <option value={5***REMOVED***>Flag</option>
                                <option value={6***REMOVED***>Academic</option>
                                <option value={7***REMOVED***>Mountain</option>
                            </Select>
                        </FormControl>
                        <FormControl>
                            <FormLabel>Select Icon Color</FormLabel>
                            <Input
                                variant="filled"
                                type="color"
                                value={coverColor***REMOVED***
                                onChange={(e) => setCovereColor(e.currentTarget.value)***REMOVED***
                                mt={3***REMOVED***
                            />
                        </FormControl>

                        <ButtonGroup display='flex' justifyContent='flex-end'>
                            <Button size='sm' variant='outline' onClick={handleClose***REMOVED***>
                                Cancel
                            </Button>
                            <Button 
                                size='sm' 
                                isDisabled={isDisabled***REMOVED*** 
                                isLoading={isLoading***REMOVED*** 
                                onClick={handleInsert***REMOVED*** 
                                colorScheme='teal'
                            >
                                Save
                            </Button>
                        </ButtonGroup>
                    </Stack>
                </PopoverContent>
            </Popover>
        </Box>
    )
***REMOVED***

export default AlbumHeader
