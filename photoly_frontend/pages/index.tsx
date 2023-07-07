import {Box, Button, Center, Container, HStack, Image, Link, Text, VStack} from "@chakra-ui/react";
import router from "next/router";
import React from "react";
import {ExternalLinkIcon} from '@chakra-ui/icons'

const Home:React.FC = () => {
  return (
      <>
          <Center h={"100vh"} bgImage={"/JH_20211030_5406.jpg"} bgSize={"cover"} position={"relative"} bgAttachment={"fixed"} bgPos={"center 0"} bgRepeat={"no-repeat"}>
          <HStack bg={"blackAlpha.300"} w={"100%"} h={16} color={"white"} p={4} position={"fixed"} top={0}
                justifyContent={"space-between"}>
            <HStack  cursor={"default"}>
                <Image src={"/favicon.ico"} alt={"logo"} width={"50px"} height={"50px"} data-testid={"logo"}/>
                <Text fontSize={"xl"} fontWeight={"medium"}>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"} p={2} spacing={4} mr={4}>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/login")}>Sign In</Button>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/signup")}>Sign Up</Button>
            </HStack>
        </HStack>
                <VStack >
                <Text fontSize='6xl' color={"whiteAlpha.900"}>See the Value of Photo Inside</Text>
                <Text fontSize='6xl' style={{marginTop:"-1rem"}} color={"orange.50"}>The Beauty Shines Here</Text>
                <Text fontSize='2xl' color={"teal.100"}>Photoly - Share Your Inspiration</Text>
                <HStack style={{marginTop:"3rem"}}>
                <Link href="/signup">
                <Button colorScheme='whiteAlpha'>
                    Sign Up
                </Button>
                </Link>
                <Link href="/login">
                <Button colorScheme='gray'>
                    Sign In
                </Button>
                </Link>
                
                </HStack>
                </VStack>
              
          </Center>

          <Center h={"100vh"} bgImage={"/JH_20220710_5856.jpg"} bgSize={"cover"} position={"relative"} bgAttachment={"fixed"} bgPos={"center 0"} bgRepeat={"no-repeat"}>
          <VStack color={"teal.900"} spacing={"2rem"}>
            <HStack>
            <Text fontSize='6xl' color={"whiteAlpha.900"} fontWeight={"bold"} fontFamily={"helvetica"}>Project Summary</Text>
            </HStack>
          
          <Container maxW='xl' bg='whiteAlpha.700' rounded={"2xl"} centerContent>
                <Box padding='4' bg='whiteAlpha.200' maxW='md'>
                    There are demanding for self-host and reliable picture host application. Photoly provides a solution just right for that. Using Photoly, you can manage
                    your pictures and photos with easy clicks, categorize them using tags, galleries, and namespaces. Moreover, Photoly provides API for you to access our 
                    functionalities for embedding in other applications.
                </Box>
            </Container>

            <Container maxW='xl' bg='whiteAlpha.700' rounded={"2xl"} centerContent>
                <Box padding='4' bg='whiteAlpha.200' maxW='md'>
                <Link href='https://github.com/WiscWebGroup/photoly' alignSelf={"flex-start"} isExternal>
                            Star our GitHub Repo! <ExternalLinkIcon mx='2px' />
                            </Link>
                </Box>
            </Container>
          </VStack>
          </Center>

          <Center h={"100vh"} bgImage={"/JH_20220601_5686.jpg"} bgSize={"cover"} position={"relative"} bgAttachment={"fixed"} bgPos={"center 0"} bgRepeat={"no-repeat"}>
          <VStack >
                <Text fontSize='6xl' color={"blackAlpha.700"} fontWeight={"bold"} fontFamily={"helvetica"}>Us. About.</Text>
                <Container maxW='2xl' centerContent >
                    <VStack spacing={"20px"} m={"20px"}>
                    <HStack bg='whiteAlpha.600' rounded={"2xl"}>
                        <Image src="https://avatars.githubusercontent.com/u/108884969"
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'
                        alt={"avatar1"}>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/WiscWebGroup' alignSelf={"flex-start"}>
                            <Text fontSize='2xl'>WiscWebGroup<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>We are WiscWebGroup, a team located in Madison, Wisconsin. We mainly develop interesting web applications.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>
                    <HStack bg='whiteAlpha.600' rounded={"2xl"}>
                        <Image src="https://avatars.githubusercontent.com/u/68500948" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'
                        alt={"avatar2"}>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/Harold-y' alignSelf={"flex-start"}>
                            <Text fontSize='2xl'>Yixuan Ye (Harold-y)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Student at UW-Madison major in CS & DS. I did the database, backend, continuous integration, and part of the frontend work of this project. I also designed the logo.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>

                    <HStack bg='whiteAlpha.600' rounded={"2xl"}>
                        <Image src="https://avatars.githubusercontent.com/u/31459252" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'
                        alt={"avatar3"}>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/Slijeff' alignSelf={"flex-start"}>
                            <Text fontSize='2xl'>Jeffrey Hui (Slijeff)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Student at UW-Madison major in CS & DS. I did part of the frontend job in this project. I photographed the background images for the home page.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>

                    <HStack bg='whiteAlpha.600' rounded={"2xl"}>
                        <Image src="https://avatars.githubusercontent.com/u/46753484" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'
                         alt={"avatar4"}>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/NataRich' alignSelf={"flex-start"}>
                            <Text fontSize='2xl'>Zihan Zhao (NataRich)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>
                    </VStack>
                    
                
            </Container>
            </VStack>
          
          </Center>
      </>


  )
}

export default Home
