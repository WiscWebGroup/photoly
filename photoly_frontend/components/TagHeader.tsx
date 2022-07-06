import { AddIcon ***REMOVED*** from "@chakra-ui/icons"
import { Box, HStack, Icon, IconButton, Text ***REMOVED*** from "@chakra-ui/react"
import { ReactNode ***REMOVED*** from "react"
import { IconType ***REMOVED*** from "react-icons"

interface TagHeaderProps {
    headerIcon?: IconType
    iconColor?: string
    onAdd?: () => void
    children?: ReactNode
***REMOVED***

const TagHeader = ({headerIcon, iconColor, onAdd, children***REMOVED***: TagHeaderProps) => {
    return (
        <Box style={{width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center'***REMOVED******REMOVED***>
            <HStack>
                <Icon as={headerIcon***REMOVED*** color={iconColor***REMOVED*** />
                <Text>{children***REMOVED***</Text>
            </HStack>
            <IconButton variant="unstyled" size="xs" aria-label="Add tag" onClick={onAdd***REMOVED*** icon={<AddIcon />***REMOVED*** />
        </Box>
    )
***REMOVED***

export default TagHeader